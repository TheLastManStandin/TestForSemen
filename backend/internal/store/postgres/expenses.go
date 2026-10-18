package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres/db"
)

var _ expense.Repository = (*Store)(nil)

func (s *Store) Insert(ctx context.Context, e domain.Expense) (domain.Expense, error) {
	row, err := s.q.InsertExpense(ctx, db.InsertExpenseParams{
		UserID: e.UserID, Amount: int64(e.Amount), Category: string(e.Category), SpentAt: e.SpentAt, Comment: e.Comment,
	})
	if err != nil {
		return domain.Expense{}, fmt.Errorf("insert expense: %w", err)
	}
	return fromRow(db.GetExpenseRow(row)), nil
}

func (s *Store) Get(ctx context.Context, id int64, owner *int64) (domain.Expense, error) {
	row, err := s.q.GetExpense(ctx, db.GetExpenseParams{ID: id, OwnerID: owner})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Expense{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Expense{}, fmt.Errorf("get expense: %w", err)
	}
	return fromRow(row), nil
}

func (s *Store) List(ctx context.Context, q expense.ListQuery) ([]domain.Expense, error) {
	limit := q.Limit
	if limit <= 0 || limit > math.MaxInt32 {
		limit = math.MaxInt32
	}
	rows, err := s.q.ListExpenses(ctx, db.ListExpensesParams{
		UserID: q.UserID, DateFrom: q.From, DateTo: q.To, Category: categoryPtr(q.Category), Text: likePattern(q.Text),
		SortRecent: q.SortRecent, Off: clampInt32(q.Offset), Lim: clampInt32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	out := make([]domain.Expense, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(db.GetExpenseRow(r)))
	}
	return out, nil
}

func (s *Store) Count(ctx context.Context, q expense.ListQuery) (int, domain.Money, error) {
	row, err := s.q.SummarizeExpenses(ctx, db.SummarizeExpensesParams{
		UserID: q.UserID, DateFrom: q.From, DateTo: q.To, Category: categoryPtr(q.Category), Text: likePattern(q.Text),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("count expenses: %w", err)
	}
	return int(row.Cnt), domain.Money(row.Total), nil
}

func (s *Store) Update(ctx context.Context, id int64, p expense.Patch, owner *int64) (domain.Expense, error) {
	var amount *int64
	if p.Amount != nil {
		amount = new(int64(*p.Amount))
	}
	row, err := s.q.UpdateExpense(ctx, db.UpdateExpenseParams{
		ID: id, UserID: p.UserID, Amount: amount, Category: categoryPtr(p.Category), SpentAt: p.SpentAt,
		Comment: p.Comment, OwnerID: owner, ExpectedVersion: p.Version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if p.Version == nil {
			return domain.Expense{}, domain.ErrNotFound
		}
		// Nothing was written; tell a stale version apart from a missing or foreign row.
		if _, err := s.Get(ctx, id, owner); err != nil {
			return domain.Expense{}, err
		}
		return domain.Expense{}, domain.ErrConflict
	}
	if err != nil {
		return domain.Expense{}, fmt.Errorf("update expense: %w", err)
	}
	return fromRow(db.GetExpenseRow(row)), nil
}

func (s *Store) Delete(ctx context.Context, id int64, owner *int64) (domain.Expense, error) {
	row, err := s.q.DeleteExpense(ctx, db.DeleteExpenseParams{ID: id, OwnerID: owner})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Expense{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Expense{}, fmt.Errorf("delete expense: %w", err)
	}
	return fromRow(db.GetExpenseRow(row)), nil
}

func (s *Store) Aggregate(ctx context.Context, q expense.AggQuery) ([]expense.AggRow, error) {
	rows, err := s.q.AggregateExpenses(ctx, db.AggregateExpensesParams{
		DateFrom: q.From, DateTo: q.To, UserID: q.UserID, Category: categoryPtr(q.Category),
	})
	if err != nil {
		return nil, fmt.Errorf("aggregate expenses: %w", err)
	}
	out := make([]expense.AggRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, expense.AggRow{
			Date: r.SpentAt, UserID: r.UserID, MemberName: r.MemberName, Category: domain.Category(r.Category),
			Total: domain.Money(r.Total), Count: int(r.Cnt),
		})
	}
	return out, nil
}

func fromRow(r db.GetExpenseRow) domain.Expense {
	return domain.Expense{
		ID: r.ID, UserID: r.UserID, MemberName: r.MemberName, Amount: domain.Money(r.Amount),
		Category: domain.Category(r.Category), SpentAt: r.SpentAt, Comment: r.Comment,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version,
	}
}

func categoryPtr(c *domain.Category) *string {
	if c == nil {
		return nil
	}
	s := string(*c)
	return &s
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// likePattern escapes LIKE wildcards so user text matches literally; empty means no filter.
func likePattern(text string) *string {
	if text == "" {
		return nil
	}
	s := likeEscaper.Replace(text)
	return &s
}

// clampInt32 maps n into [0, MaxInt32] so the conversion cannot overflow.
func clampInt32(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}
