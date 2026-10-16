// Package httpapi adapts expense.Service to the REST contract in api/openapi.yaml.
package httpapi

import (
	"context"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

// TraceLister reads the agent journal (implemented by postgres.Store).
type TraceLister interface {
	ListTraces(ctx context.Context, userID *int64, limit, offset int) ([]domain.Trace, int, error)
}

// Server implements the generated strict interface. Handlers return errors;
// errorHandler turns them into RFC 7807 responses.
type Server struct {
	svc    *expense.Service
	traces TraceLister
}

var _ gen.StrictServerInterface = (*Server)(nil)

func (s *Server) ListMembers(ctx context.Context, _ gen.ListMembersRequestObject) (gen.ListMembersResponseObject, error) {
	members, err := s.svc.ListMembers(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListMembers200JSONResponse, 0, len(members))
	for _, m := range members {
		out = append(out, gen.Member{TelegramId: m.TelegramID, Name: m.Name})
	}
	return out, nil
}

func (s *Server) ListExpenses(ctx context.Context, req gen.ListExpensesRequestObject) (gen.ListExpensesResponseObject, error) {
	p := req.Params
	cat, err := parseCategory(p.Category)
	if err != nil {
		return nil, err
	}
	page, err := s.svc.List(ctx, expense.All(), expense.ListQuery{
		UserID: p.MemberId, From: dateOrNil(p.From), To: dateOrNil(p.To), Category: cat,
		Limit: intOr(p.Limit, expense.DefaultLimit), Offset: intOr(p.Offset, 0),
	})
	if err != nil {
		return nil, err
	}
	items := make([]gen.Expense, 0, len(page.Items))
	for _, e := range page.Items {
		items = append(items, toAPIExpense(e))
	}
	return gen.ListExpenses200JSONResponse{Items: items, Total: page.Total, Sum: page.Sum.String()}, nil
}

func (s *Server) CreateExpense(ctx context.Context, req gen.CreateExpenseRequestObject) (gen.CreateExpenseResponseObject, error) {
	b := req.Body
	if b == nil {
		return nil, domain.Invalid("body", "пустое тело запроса")
	}
	amount, err := parseAmount(b.Amount)
	if err != nil {
		return nil, err
	}
	cat, err := domain.ParseCategory(string(b.Category))
	if err != nil {
		return nil, err
	}
	if b.SpentAt.IsZero() {
		return nil, domain.Invalid("spent_at", "дата траты обязательна")
	}
	e, err := s.svc.Create(ctx, expense.CreateInput{
		UserID: b.UserId, Amount: amount, Category: cat, SpentAt: b.SpentAt.Time, Comment: deref(b.Comment),
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateExpense201JSONResponse(toAPIExpense(e)), nil
}

func (s *Server) GetExpense(ctx context.Context, req gen.GetExpenseRequestObject) (gen.GetExpenseResponseObject, error) {
	e, err := s.svc.Get(ctx, expense.All(), req.Id)
	if err != nil {
		return nil, err
	}
	return gen.GetExpense200JSONResponse(toAPIExpense(e)), nil
}

func (s *Server) UpdateExpense(ctx context.Context, req gen.UpdateExpenseRequestObject) (gen.UpdateExpenseResponseObject, error) {
	b := req.Body
	if b == nil {
		return nil, domain.Invalid("body", "пустое тело запроса")
	}
	patch := expense.Patch{UserID: b.UserId, Comment: b.Comment, Version: b.Version}
	if b.Amount != nil {
		m, err := parseAmount(*b.Amount)
		if err != nil {
			return nil, err
		}
		patch.Amount = &m
	}
	if b.Category != nil {
		c, err := domain.ParseCategory(string(*b.Category))
		if err != nil {
			return nil, err
		}
		patch.Category = &c
	}
	if b.SpentAt != nil {
		t := b.SpentAt.Time
		patch.SpentAt = &t
	}
	e, err := s.svc.Update(ctx, expense.All(), req.Id, patch)
	if err != nil {
		return nil, err
	}
	return gen.UpdateExpense200JSONResponse(toAPIExpense(e)), nil
}

func (s *Server) DeleteExpense(ctx context.Context, req gen.DeleteExpenseRequestObject) (gen.DeleteExpenseResponseObject, error) {
	if _, err := s.svc.Delete(ctx, expense.All(), req.Id); err != nil {
		return nil, err
	}
	return gen.DeleteExpense204Response{}, nil
}

func (s *Server) GetStats(ctx context.Context, req gen.GetStatsRequestObject) (gen.GetStatsResponseObject, error) {
	p := req.Params
	cat, err := parseCategory(p.Category)
	if err != nil {
		return nil, err
	}
	st, err := s.svc.Stats(ctx, expense.StatsQuery{From: p.From.Time, To: p.To.Time, UserID: p.MemberId, Category: cat})
	if err != nil {
		return nil, err
	}
	return gen.GetStats200JSONResponse(toAPIStats(st)), nil
}

func (s *Server) ListTraces(ctx context.Context, req gen.ListTracesRequestObject) (gen.ListTracesResponseObject, error) {
	p := req.Params
	limit := intOr(p.Limit, 20)
	if limit < 1 || limit > 100 {
		limit = 20
	}
	traces, total, err := s.traces.ListTraces(ctx, p.MemberId, limit, max(intOr(p.Offset, 0), 0))
	if err != nil {
		return nil, err
	}
	items := make([]gen.Trace, 0, len(traces))
	for _, t := range traces {
		items = append(items, toAPITrace(t))
	}
	return gen.ListTraces200JSONResponse{Items: items, Total: total}, nil
}
