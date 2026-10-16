// Package expense holds the business rules for family expenses.
package expense

import (
	"context"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// ListQuery filters and pages expenses. Nil pointers mean "no filter".
type ListQuery struct {
	UserID     *int64
	From, To   *time.Time // inclusive domain dates
	Category   *domain.Category
	Text       string // case-insensitive substring of the comment
	SortRecent bool   // newest created first, instead of newest spent first
	Limit      int    // <= 0 means no limit (Service always sets one)
	Offset     int
}

// AggQuery selects expenses for aggregation over an inclusive date range.
type AggQuery struct {
	From, To time.Time
	UserID   *int64
	Category *domain.Category
}

// AggRow is a total grouped by (date, user, category).
type AggRow struct {
	Date       time.Time
	UserID     int64
	MemberName string
	Category   domain.Category
	Total      domain.Money
	Count      int
}

// Repository persists expenses and members.
// Implementations: store/postgres (production) and expense/memrepo (tests, evals),
// both verified by expensetest.RunRepositoryContract.
type Repository interface {
	UpsertMembers(ctx context.Context, members []domain.Member) error
	// ListMembers returns members ordered by name.
	ListMembers(ctx context.Context) ([]domain.Member, error)
	MemberExists(ctx context.Context, telegramID int64) (bool, error)

	// Insert stores e and fills ID, MemberName, CreatedAt, UpdatedAt and Version (1).
	Insert(ctx context.Context, e domain.Expense) (domain.Expense, error)
	// Get returns domain.ErrNotFound if the expense is missing or owner != nil and it belongs to someone else.
	Get(ctx context.Context, id int64, owner *int64) (domain.Expense, error)
	// List orders by created_at desc (only if SortRecent), then spent_at desc, then id desc.
	List(ctx context.Context, q ListQuery) ([]domain.Expense, error)
	// Count returns the number and sum of expenses matching q, ignoring Limit and Offset.
	Count(ctx context.Context, q ListQuery) (int, domain.Money, error)
	// Update sets only the non-nil fields of p (already normalized by Service), increments
	// Version and refreshes UpdatedAt in one atomic write, so concurrent changes to other
	// fields survive. All checks run against the current row in the same write:
	// domain.ErrNotFound if the expense is missing or owner != nil and it belongs to someone
	// else; domain.ErrConflict if p.Version != nil and differs from the stored version.
	// Nothing is written when an error is returned. Service never passes a patch without fields.
	Update(ctx context.Context, id int64, p Patch, owner *int64) (domain.Expense, error)
	// Delete returns domain.ErrNotFound if the expense is missing or owner != nil and it belongs to someone else.
	Delete(ctx context.Context, id int64, owner *int64) error
	// Aggregate groups by (date, user, category) and orders rows the same way.
	Aggregate(ctx context.Context, q AggQuery) ([]AggRow, error)
}
