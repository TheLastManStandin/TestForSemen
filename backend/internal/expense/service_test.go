package expense_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/memrepo"
)

var (
	papa = expensetest.Papa.TelegramID
	mama = expensetest.Mama.TelegramID
	// Thursday 2026-10-15 12:00 Moscow.
	fixedNow = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	ctx      = context.Background()
)

type recorder struct {
	mu     sync.Mutex
	events []events.Event
}

func (r *recorder) Publish(e events.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) types() []events.Type {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]events.Type, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.Type)
	}
	return out
}

func newService(t *testing.T, now time.Time) (*expense.Service, *recorder) {
	t.Helper()
	repo := memrepo.New()
	require.NoError(t, repo.UpsertMembers(ctx, []domain.Member{expensetest.Papa, expensetest.Mama}))
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	rec := &recorder{}
	return expense.NewService(repo, rec, loc, func() time.Time { return now }), rec
}

func mustCreate(t *testing.T, svc *expense.Service, user int64, amount domain.Money, cat domain.Category, date, comment string) domain.Expense {
	t.Helper()
	e, err := svc.Create(ctx, expense.CreateInput{UserID: user, Amount: amount, Category: cat, SpentAt: expensetest.Date(date), Comment: comment})
	require.NoError(t, err)
	return e
}

func fieldOf(t *testing.T, err error) string {
	t.Helper()
	var ve *domain.ValidationError
	require.True(t, errors.As(err, &ve), "want ValidationError, got %v", err)
	return ve.Field
}

func ptr[T any](v T) *T { return &v }

func TestTodayUsesLocation(t *testing.T) {
	svc, _ := newService(t, time.Date(2026, 10, 15, 22, 30, 0, 0, time.UTC))
	assert.Equal(t, "2026-10-16", domain.FormatDate(svc.Today()))
}

func TestCreateDefaultsAndPublishes(t *testing.T) {
	svc, rec := newService(t, fixedNow)
	e, err := svc.Create(ctx, expense.CreateInput{UserID: papa, Amount: 35000, Category: domain.CategoryFood, Comment: "  кофе  "})
	require.NoError(t, err)
	assert.Equal(t, "2026-10-15", domain.FormatDate(e.SpentAt))
	assert.Equal(t, "кофе", e.Comment)
	assert.Equal(t, "Папа", e.MemberName)
	assert.Equal(t, []events.Type{events.ExpenseCreated}, rec.types())
}

func TestCreateValidation(t *testing.T) {
	svc, rec := newService(t, fixedNow)
	valid := expense.CreateInput{UserID: papa, Amount: 100, Category: domain.CategoryFood, SpentAt: expensetest.Date("2026-10-15")}
	cases := []struct {
		name, field string
		mutate      func(in *expense.CreateInput)
	}{
		{"zero amount", "amount", func(in *expense.CreateInput) { in.Amount = 0 }},
		{"huge amount", "amount", func(in *expense.CreateInput) { in.Amount = expense.MaxAmount + 1 }},
		{"unknown category", "category", func(in *expense.CreateInput) { in.Category = "pets" }},
		{"future date", "spent_at", func(in *expense.CreateInput) { in.SpentAt = expensetest.Date("2026-10-17") }},
		{"long comment", "comment", func(in *expense.CreateInput) { in.Comment = strings.Repeat("я", expense.MaxComment+1) }},
		{"stranger", "user_id", func(in *expense.CreateInput) { in.UserID = 999 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := valid
			c.mutate(&in)
			_, err := svc.Create(ctx, in)
			assert.Equal(t, c.field, fieldOf(t, err))
		})
	}
	assert.Empty(t, rec.types(), "failed creates publish nothing")

	tomorrow := valid
	tomorrow.SpentAt = expensetest.Date("2026-10-16")
	_, err := svc.Create(ctx, tomorrow)
	assert.NoError(t, err, "tomorrow is tolerated (timezone slack)")
}

func TestOwnScopeIsolation(t *testing.T) {
	svc, _ := newService(t, fixedNow)
	mamas := mustCreate(t, svc, mama, 500000, domain.CategoryShopping, "2026-10-12", "кроссовки")

	_, err := svc.Get(ctx, expense.Own(papa), mamas.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = svc.Update(ctx, expense.Own(papa), mamas.ID, expense.Patch{Amount: ptr(domain.Money(1))})
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = svc.Delete(ctx, expense.Own(papa), mamas.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	page, err := svc.List(ctx, expense.Own(papa), expense.ListQuery{})
	require.NoError(t, err)
	assert.Equal(t, 0, page.Total)

	got, err := svc.Get(ctx, expense.All(), mamas.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.Money(500000), got.Amount, "mama's expense is untouched")
}

func TestListOwnScopeOverridesFilter(t *testing.T) {
	svc, _ := newService(t, fixedNow)
	mustCreate(t, svc, papa, 100, domain.CategoryFood, "2026-10-14", "")
	mustCreate(t, svc, mama, 200, domain.CategoryFood, "2026-10-14", "")

	page, err := svc.List(ctx, expense.Own(papa), expense.ListQuery{UserID: ptr(mama)})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, papa, page.Items[0].UserID)
	assert.Equal(t, domain.Money(100), page.Sum)
}

func TestListDefaultsAndValidation(t *testing.T) {
	svc, _ := newService(t, fixedNow)
	for range 3 {
		mustCreate(t, svc, papa, 100, domain.CategoryFood, "2026-10-14", "")
	}
	page, err := svc.List(ctx, expense.All(), expense.ListQuery{})
	require.NoError(t, err)
	assert.Len(t, page.Items, 3)
	assert.Equal(t, 3, page.Total)

	_, err = svc.List(ctx, expense.All(), expense.ListQuery{From: ptr(expensetest.Date("2026-10-15")), To: ptr(expensetest.Date("2026-10-01"))})
	assert.Equal(t, "to", fieldOf(t, err))
}

func TestUpdate(t *testing.T) {
	svc, rec := newService(t, fixedNow)
	e := mustCreate(t, svc, papa, 35000, domain.CategoryFood, "2026-10-14", "кофе")

	upd, err := svc.Update(ctx, expense.Own(papa), e.ID, expense.Patch{Amount: ptr(domain.Money(45000)), Comment: ptr("")})
	require.NoError(t, err)
	assert.Equal(t, domain.Money(45000), upd.Amount)
	assert.Empty(t, upd.Comment)
	assert.Equal(t, domain.CategoryFood, upd.Category, "unpatched fields are kept")

	_, err = svc.Update(ctx, expense.Own(papa), e.ID, expense.Patch{UserID: ptr(mama)})
	assert.Equal(t, "user_id", fieldOf(t, err), "a member cannot hand an expense to someone else")

	_, err = svc.Update(ctx, expense.Own(papa), e.ID, expense.Patch{Amount: ptr(domain.Money(0))})
	assert.Equal(t, "amount", fieldOf(t, err))

	moved, err := svc.Update(ctx, expense.All(), e.ID, expense.Patch{UserID: ptr(mama)})
	require.NoError(t, err)
	assert.Equal(t, "Мама", moved.MemberName)

	assert.Equal(t, []events.Type{events.ExpenseCreated, events.ExpenseUpdated, events.ExpenseUpdated}, rec.types())
}

func TestUpdateRejectsEmptyPatch(t *testing.T) {
	svc, rec := newService(t, fixedNow)
	e := mustCreate(t, svc, papa, 35000, domain.CategoryFood, "2026-10-14", "кофе")

	for _, p := range []expense.Patch{{}, {Version: ptr(e.Version)}} {
		_, err := svc.Update(ctx, expense.All(), e.ID, p)
		assert.Equal(t, "fields", fieldOf(t, err))
	}
	got, err := svc.Get(ctx, expense.All(), e.ID)
	require.NoError(t, err)
	assert.Equal(t, e.Version, got.Version, "nothing written")
	assert.True(t, got.UpdatedAt.Equal(e.UpdatedAt))
	assert.Equal(t, []events.Type{events.ExpenseCreated}, rec.types(), "nothing published")
}

func TestDelete(t *testing.T) {
	svc, rec := newService(t, fixedNow)
	e := mustCreate(t, svc, papa, 120000, domain.CategoryTransport, "2026-10-14", "такси")

	deleted, err := svc.Delete(ctx, expense.Own(papa), e.ID)
	require.NoError(t, err)
	assert.Equal(t, "такси", deleted.Comment)
	_, err = svc.Get(ctx, expense.All(), e.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, []events.Type{events.ExpenseCreated, events.ExpenseDeleted}, rec.types())
}
