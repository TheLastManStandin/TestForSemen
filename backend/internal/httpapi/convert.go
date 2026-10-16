package httpapi

import (
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

func toAPIExpense(e domain.Expense) gen.Expense {
	return gen.Expense{
		Id: e.ID, UserId: e.UserID, MemberName: e.MemberName, Amount: e.Amount.String(),
		Category: gen.Category(e.Category), SpentAt: openapi_types.Date{Time: e.SpentAt},
		Comment: strPtrOrNil(e.Comment), CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, Version: e.Version,
	}
}

func toAPIStats(s expense.Stats) gen.Stats {
	out := gen.Stats{
		Total: s.Total.String(), PrevTotal: s.PrevTotal.String(), Count: s.Count,
		ByCategory: make([]gen.CategoryTotal, 0, len(s.ByCategory)),
		ByMember:   make([]gen.MemberTotal, 0, len(s.ByMember)),
		ByDay:      make([]gen.DayTotal, 0, len(s.ByDay)),
	}
	for _, c := range s.ByCategory {
		out.ByCategory = append(out.ByCategory, gen.CategoryTotal{Category: gen.Category(c.Category), Total: c.Total.String(), Count: c.Count})
	}
	for _, m := range s.ByMember {
		out.ByMember = append(out.ByMember, gen.MemberTotal{UserId: m.UserID, MemberName: m.MemberName, Total: m.Total.String(), Count: m.Count})
	}
	for _, d := range s.ByDay {
		out.ByDay = append(out.ByDay, gen.DayTotal{Date: openapi_types.Date{Time: d.Date}, UserId: d.UserID, Total: d.Total.String()})
	}
	return out
}

func toAPITrace(t domain.Trace) gen.Trace {
	steps := make([]gen.TraceStep, 0, len(t.Steps))
	for _, s := range t.Steps {
		steps = append(steps, gen.TraceStep{Tool: s.Tool, Args: s.Args, Result: s.Result, Ms: s.Ms})
	}
	return gen.Trace{
		Id: t.ID, UserId: t.UserID, MemberName: t.MemberName, Source: gen.TraceSource(t.Source), Input: t.Input,
		Output: strPtrOrNil(t.Output), Steps: steps, Model: t.Model, PromptTokens: t.PromptTokens,
		CompletionTokens: t.CompletionTokens, SttMs: t.STTms, LatencyMs: t.LatencyMs,
		Error: strPtrOrNil(t.Error), CreatedAt: t.CreatedAt,
	}
}

func parseCategory(c *gen.Category) (*domain.Category, error) {
	if c == nil {
		return nil, nil
	}
	cat, err := domain.ParseCategory(string(*c))
	if err != nil {
		return nil, err
	}
	return &cat, nil
}

func parseAmount(s string) (domain.Money, error) {
	m, err := domain.ParseMoney(s)
	if err != nil {
		return 0, domain.Invalid("amount", "некорректная сумма")
	}
	return m, nil
}

func dateOrNil(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}

func intOr(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
