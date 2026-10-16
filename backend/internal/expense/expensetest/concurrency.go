package expensetest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
)

// interleaved runs hook once, right after the first Get returns, before the caller writes.
// It deterministically reproduces "another client changed the row between my read and my write".
type interleaved struct {
	expense.Repository
	once sync.Once
	hook func()
}

func (r *interleaved) Get(ctx context.Context, id int64, owner *int64) (domain.Expense, error) {
	e, err := r.Repository.Get(ctx, id, owner)
	r.once.Do(r.hook)
	return e, err
}

// RunUpdateConcurrency verifies that independent concurrent edits made through the Service
// (bot without a version, panel with one) never silently overwrite each other.
func RunUpdateConcurrency(t *testing.T, newRepo Factory) {
	ctx := context.Background()
	now := func() time.Time { return time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC) }

	setup := func(t *testing.T) (expense.Repository, domain.Expense) {
		t.Helper()
		r := newRepo(t)
		require.NoError(t, r.UpsertMembers(ctx, []domain.Member{Papa, Mama}))
		e, err := r.Insert(ctx, domain.Expense{UserID: Papa.TelegramID, Amount: 35000, Category: domain.CategoryFood, SpentAt: Date("2026-10-10"), Comment: "кофе"})
		require.NoError(t, err)
		return r, e
	}

	t.Run("partial updates from two paths both survive", func(t *testing.T) {
		raw, e := setup(t)
		other := expense.NewService(raw, nil, time.UTC, now)
		var hookErr error
		wrapped := &interleaved{Repository: raw, hook: func() {
			// Path B (panel) changes the comment while path A (bot) holds a stale read.
			_, hookErr = other.Update(ctx, expense.All(), e.ID, expense.Patch{Comment: ptr("капучино")})
		}}
		bot := expense.NewService(wrapped, nil, time.UTC, now)

		_, err := bot.Update(ctx, expense.Own(Papa.TelegramID), e.ID, expense.Patch{Amount: ptr(domain.Money(45000))})
		require.NoError(t, err)
		require.NoError(t, hookErr)

		got, err := raw.Get(ctx, e.ID, nil)
		require.NoError(t, err)
		assert.Equal(t, domain.Money(45000), got.Amount, "path A change kept")
		assert.Equal(t, "капучино", got.Comment, "path B change kept")
	})

	t.Run("same stale version: first wins, second conflicts", func(t *testing.T) {
		raw, e := setup(t)
		svc := expense.NewService(raw, nil, time.UTC, now)

		first, err := svc.Update(ctx, expense.All(), e.ID, expense.Patch{Amount: ptr(domain.Money(45000)), Version: ptr(e.Version)})
		require.NoError(t, err)
		assert.Equal(t, e.Version+1, first.Version)

		_, err = svc.Update(ctx, expense.All(), e.ID, expense.Patch{Comment: ptr("чай"), Version: ptr(e.Version)})
		require.ErrorIs(t, err, domain.ErrConflict)

		got, err := raw.Get(ctx, e.ID, nil)
		require.NoError(t, err)
		assert.Equal(t, domain.Money(45000), got.Amount)
		assert.Equal(t, "кофе", got.Comment, "the stale update wrote nothing")
		assert.Equal(t, first.Version, got.Version)
	})

	t.Run("version check is atomic with the write", func(t *testing.T) {
		raw, e := setup(t)
		other := expense.NewService(raw, nil, time.UTC, now)
		var hookErr error
		wrapped := &interleaved{Repository: raw, hook: func() {
			// Another panel tab saves after this request has already read version 1.
			_, hookErr = other.Update(ctx, expense.All(), e.ID, expense.Patch{Comment: ptr("капучино"), Version: ptr(e.Version)})
		}}
		panel := expense.NewService(wrapped, nil, time.UTC, now)

		_, err := panel.Update(ctx, expense.All(), e.ID, expense.Patch{Amount: ptr(domain.Money(45000)), Version: ptr(e.Version)})
		require.ErrorIs(t, err, domain.ErrConflict)
		require.NoError(t, hookErr)

		got, err := raw.Get(ctx, e.ID, nil)
		require.NoError(t, err)
		assert.Equal(t, domain.Money(35000), got.Amount)
		assert.Equal(t, "капучино", got.Comment)
		assert.Equal(t, e.Version+1, got.Version)
	})

	t.Run("parallel updates with one version: exactly one succeeds", func(t *testing.T) {
		raw, e := setup(t)
		svc := expense.NewService(raw, nil, time.UTC, now)
		const n = 8
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := range n {
			wg.Go(func() {
				_, errs[i] = svc.Update(ctx, expense.All(), e.ID, expense.Patch{Amount: ptr(domain.Money(40000 + i)), Version: ptr(e.Version)})
			})
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
				continue
			}
			assert.ErrorIs(t, err, domain.ErrConflict)
		}
		assert.Equal(t, 1, ok)
		got, err := raw.Get(ctx, e.ID, nil)
		require.NoError(t, err)
		assert.Equal(t, e.Version+1, got.Version)
	})

	t.Run("parallel partial updates without version all apply", func(t *testing.T) {
		raw, e := setup(t)
		svc := expense.NewService(raw, nil, time.UTC, now)
		var wg sync.WaitGroup
		var errA, errB error
		wg.Go(func() {
			_, errA = svc.Update(ctx, expense.Own(Papa.TelegramID), e.ID, expense.Patch{Amount: ptr(domain.Money(45000))})
		})
		wg.Go(func() {
			_, errB = svc.Update(ctx, expense.All(), e.ID, expense.Patch{Category: ptr(domain.CategoryOther)})
		})
		wg.Wait()
		require.NoError(t, errA)
		require.NoError(t, errB)
		got, err := raw.Get(ctx, e.ID, nil)
		require.NoError(t, err)
		assert.Equal(t, domain.Money(45000), got.Amount)
		assert.Equal(t, domain.CategoryOther, got.Category)
		assert.Equal(t, e.Version+2, got.Version)
	})

	t.Run("foreign owner partial update is not found and changes nothing", func(t *testing.T) {
		raw, e := setup(t)
		svc := expense.NewService(raw, nil, time.UTC, now)
		_, err := svc.Update(ctx, expense.Own(Mama.TelegramID), e.ID, expense.Patch{Comment: ptr("чужое")})
		require.ErrorIs(t, err, domain.ErrNotFound)
		got, err := raw.Get(ctx, e.ID, nil)
		require.NoError(t, err)
		assert.Equal(t, "кофе", got.Comment)
		assert.Equal(t, e.Version, got.Version)
	})
}
