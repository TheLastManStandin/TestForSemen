// Package expensetest provides a shared behavioural contract for expense.Repository implementations.
package expensetest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
)

// Test family.
var (
	Papa = domain.Member{TelegramID: 111, Name: "Папа"}
	Mama = domain.Member{TelegramID: 222, Name: "Мама"}
)

// Factory returns an empty repository.
type Factory func(t *testing.T) expense.Repository

// Date parses "YYYY-MM-DD" and panics on error (test helper).
func Date(s string) time.Time {
	t, err := time.Parse(domain.DateLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ptr[T any](v T) *T { return &v }

type aggKey struct {
	Date     string
	UserID   int64
	Member   string
	Category domain.Category
	Total    domain.Money
	Count    int
}

// RunRepositoryContract runs the behaviour every Repository implementation must satisfy.
func RunRepositoryContract(t *testing.T, newRepo Factory) {
	ctx := context.Background()

	setup := func(t *testing.T) expense.Repository {
		t.Helper()
		r := newRepo(t)
		require.NoError(t, r.UpsertMembers(ctx, []domain.Member{Papa, Mama}))
		return r
	}
	insert := func(t *testing.T, r expense.Repository, user int64, amount domain.Money, cat domain.Category, date, comment string) domain.Expense {
		t.Helper()
		e, err := r.Insert(ctx, domain.Expense{UserID: user, Amount: amount, Category: cat, SpentAt: Date(date), Comment: comment})
		require.NoError(t, err)
		return e
	}
	// fixture inserts e1..e4 in this order.
	fixture := func(t *testing.T, r expense.Repository) (e1, e2, e3, e4 domain.Expense) {
		t.Helper()
		e1 = insert(t, r, Papa.TelegramID, 35000, domain.CategoryFood, "2026-10-10", "кофе")
		e2 = insert(t, r, Papa.TelegramID, 120000, domain.CategoryTransport, "2026-10-14", "такси")
		e3 = insert(t, r, Mama.TelegramID, 500000, domain.CategoryShopping, "2026-10-12", "кроссовки")
		e4 = insert(t, r, Papa.TelegramID, 90000, domain.CategoryTransport, "2026-10-14", "Такси домой")
		return
	}
	ids := func(items []domain.Expense) []int64 {
		out := make([]int64, 0, len(items))
		for _, e := range items {
			out = append(out, e.ID)
		}
		return out
	}
	list := func(t *testing.T, r expense.Repository, q expense.ListQuery) []int64 {
		t.Helper()
		if q.Limit == 0 {
			q.Limit = 50
		}
		items, err := r.List(ctx, q)
		require.NoError(t, err)
		return ids(items)
	}

	t.Run("members", func(t *testing.T) {
		r := setup(t)
		members, err := r.ListMembers(ctx)
		require.NoError(t, err)
		assert.Equal(t, []domain.Member{Mama, Papa}, members)

		renamed := domain.Member{TelegramID: Papa.TelegramID, Name: "Папа Вова"}
		require.NoError(t, r.UpsertMembers(ctx, []domain.Member{renamed}))
		members, err = r.ListMembers(ctx)
		require.NoError(t, err)
		assert.Equal(t, []domain.Member{Mama, renamed}, members)

		ok, err := r.MemberExists(ctx, Papa.TelegramID)
		require.NoError(t, err)
		assert.True(t, ok)
		ok, err = r.MemberExists(ctx, 999)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("insert and get", func(t *testing.T) {
		r := setup(t)
		e := insert(t, r, Papa.TelegramID, 35000, domain.CategoryFood, "2026-10-10", "кофе")
		assert.Positive(t, e.ID)
		assert.Equal(t, "Папа", e.MemberName)
		assert.Equal(t, domain.Money(35000), e.Amount)
		assert.Equal(t, domain.CategoryFood, e.Category)
		assert.Equal(t, "2026-10-10", domain.FormatDate(e.SpentAt))
		assert.Equal(t, "кофе", e.Comment)
		assert.False(t, e.CreatedAt.IsZero())
		assert.True(t, e.UpdatedAt.Equal(e.CreatedAt))

		got, err := r.Get(ctx, e.ID, nil)
		require.NoError(t, err)
		assert.Equal(t, e.ID, got.ID)
		assert.Equal(t, e.Amount, got.Amount)
		assert.Equal(t, "Папа", got.MemberName)
		assert.Equal(t, "2026-10-10", domain.FormatDate(got.SpentAt))
		assert.Equal(t, "кофе", got.Comment)
	})

	t.Run("get respects owner", func(t *testing.T) {
		r := setup(t)
		e := insert(t, r, Papa.TelegramID, 100, domain.CategoryOther, "2026-10-10", "")

		_, err := r.Get(ctx, e.ID, ptr(Mama.TelegramID))
		assert.ErrorIs(t, err, domain.ErrNotFound)
		_, err = r.Get(ctx, e.ID, ptr(Papa.TelegramID))
		assert.NoError(t, err)
		_, err = r.Get(ctx, 9999, nil)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("list filters and order", func(t *testing.T) {
		r := setup(t)
		e1, e2, e3, e4 := fixture(t, r)

		assert.Equal(t, []int64{e4.ID, e2.ID, e3.ID, e1.ID}, list(t, r, expense.ListQuery{}))
		assert.Equal(t, []int64{e4.ID, e2.ID, e1.ID}, list(t, r, expense.ListQuery{UserID: ptr(Papa.TelegramID)}))
		assert.Equal(t, []int64{e3.ID}, list(t, r, expense.ListQuery{From: ptr(Date("2026-10-12")), To: ptr(Date("2026-10-13"))}))
		assert.Equal(t, []int64{e4.ID, e2.ID}, list(t, r, expense.ListQuery{Category: ptr(domain.CategoryTransport)}))
		// Case-insensitive Cyrillic search.
		assert.Equal(t, []int64{e4.ID, e2.ID}, list(t, r, expense.ListQuery{Text: "такси"}))
		assert.Equal(t, []int64{e4.ID, e2.ID}, list(t, r, expense.ListQuery{Text: "ТАКСИ"}))
		assert.Equal(t, []int64{e4.ID, e3.ID, e2.ID, e1.ID}, list(t, r, expense.ListQuery{SortRecent: true}))
		assert.Equal(t, []int64{e2.ID, e3.ID}, list(t, r, expense.ListQuery{Limit: 2, Offset: 1}))
		assert.Empty(t, list(t, r, expense.ListQuery{Offset: 10}))
	})

	t.Run("count ignores pagination", func(t *testing.T) {
		r := setup(t)
		fixture(t, r)

		n, sum, err := r.Count(ctx, expense.ListQuery{Limit: 1, Offset: 3})
		require.NoError(t, err)
		assert.Equal(t, 4, n)
		assert.Equal(t, domain.Money(745000), sum)

		n, sum, err = r.Count(ctx, expense.ListQuery{Text: "такси"})
		require.NoError(t, err)
		assert.Equal(t, 2, n)
		assert.Equal(t, domain.Money(210000), sum)

		n, sum, err = r.Count(ctx, expense.ListQuery{UserID: ptr(Mama.TelegramID)})
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, domain.Money(500000), sum)

		// LIKE wildcards in user text are literal.
		n, sum, err = r.Count(ctx, expense.ListQuery{Text: "%"})
		require.NoError(t, err)
		assert.Equal(t, 0, n)
		assert.Equal(t, domain.Money(0), sum)
	})

	t.Run("update", func(t *testing.T) {
		r := setup(t)
		e := insert(t, r, Papa.TelegramID, 35000, domain.CategoryFood, "2026-10-10", "кофе")

		e.Amount = 45000
		e.Category = domain.CategoryOther
		e.SpentAt = Date("2026-10-11")
		e.Comment = "обед"
		e.UserID = Mama.TelegramID
		upd, err := r.Update(ctx, e)
		require.NoError(t, err)
		assert.Equal(t, domain.Money(45000), upd.Amount)
		assert.Equal(t, domain.CategoryOther, upd.Category)
		assert.Equal(t, "2026-10-11", domain.FormatDate(upd.SpentAt))
		assert.Equal(t, "обед", upd.Comment)
		assert.Equal(t, Mama.TelegramID, upd.UserID)
		assert.Equal(t, "Мама", upd.MemberName)
		assert.False(t, upd.UpdatedAt.Before(upd.CreatedAt))

		got, err := r.Get(ctx, e.ID, nil)
		require.NoError(t, err)
		assert.Equal(t, domain.Money(45000), got.Amount)

		_, err = r.Update(ctx, domain.Expense{ID: 9999, UserID: Papa.TelegramID, Amount: 1, Category: domain.CategoryFood, SpentAt: Date("2026-10-10")})
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("delete", func(t *testing.T) {
		r := setup(t)
		e := insert(t, r, Papa.TelegramID, 100, domain.CategoryOther, "2026-10-10", "")

		require.NoError(t, r.Delete(ctx, e.ID))
		_, err := r.Get(ctx, e.ID, nil)
		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.ErrorIs(t, r.Delete(ctx, e.ID), domain.ErrNotFound)
	})

	t.Run("aggregate", func(t *testing.T) {
		r := setup(t)
		fixture(t, r)
		agg := func(q expense.AggQuery) []aggKey {
			t.Helper()
			rows, err := r.Aggregate(ctx, q)
			require.NoError(t, err)
			out := make([]aggKey, 0, len(rows))
			for _, row := range rows {
				out = append(out, aggKey{domain.FormatDate(row.Date), row.UserID, row.MemberName, row.Category, row.Total, row.Count})
			}
			return out
		}

		all := expense.AggQuery{From: Date("2026-10-10"), To: Date("2026-10-14")}
		assert.Equal(t, []aggKey{
			{"2026-10-10", Papa.TelegramID, "Папа", domain.CategoryFood, 35000, 1},
			{"2026-10-12", Mama.TelegramID, "Мама", domain.CategoryShopping, 500000, 1},
			{"2026-10-14", Papa.TelegramID, "Папа", domain.CategoryTransport, 210000, 2},
		}, agg(all))

		papaOnly := all
		papaOnly.UserID = ptr(Papa.TelegramID)
		assert.Len(t, agg(papaOnly), 2)

		transport := all
		transport.Category = ptr(domain.CategoryTransport)
		assert.Len(t, agg(transport), 1)

		assert.Len(t, agg(expense.AggQuery{From: Date("2026-10-11"), To: Date("2026-10-13")}), 1)
	})
}
