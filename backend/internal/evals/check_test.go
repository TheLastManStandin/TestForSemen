package evals

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/memrepo"
)

var today = domain.DateOf(Now)

func seeded(t *testing.T) map[string]domain.Expense {
	t.Helper()
	repo := memrepo.New()
	require.NoError(t, repo.UpsertMembers(context.Background(), Members))
	svc := expense.NewService(repo, nil, Now.Location(), func() time.Time { return Now })
	s, err := Seed(context.Background(), svc)
	require.NoError(t, err)
	return s
}

func coffee(amount domain.Money, date time.Time) domain.Expense {
	return domain.Expense{ID: 100, UserID: User.TelegramID, Amount: amount, Category: domain.CategoryFood, SpentAt: date, Comment: "Кофе"}
}

func TestComputeDiff(t *testing.T) {
	a := domain.Expense{ID: 1, Amount: 100}
	b := domain.Expense{ID: 2, Amount: 200}
	b2 := b
	b2.Amount = 250
	c := domain.Expense{ID: 3, Amount: 300}
	d := ComputeDiff([]domain.Expense{a, b}, []domain.Expense{b2, c})
	require.Len(t, d.Created, 1)
	assert.Equal(t, int64(3), d.Created[0].ID)
	require.Len(t, d.Updated, 1)
	assert.Equal(t, domain.Money(250), d.Updated[0].After.Amount)
	require.Len(t, d.Deleted, 1)
	assert.Equal(t, int64(1), d.Deleted[0].ID)
	assert.True(t, ComputeDiff([]domain.Expense{a}, []domain.Expense{a}).Empty())
}

func TestCheckCreated(t *testing.T) {
	s := seeded(t)
	exp := Expect{Created: []ExpenseMatch{{Amount: "350", Category: "food", Date: "today", Comment: "кофе"}}}

	ok := Outcome{Diff: Diff{Created: []domain.Expense{coffee(35000, today)}}, Reply: "Записал"}
	assert.Empty(t, Check(exp, s, ok, today))

	wrongAmount := Outcome{Diff: Diff{Created: []domain.Expense{coffee(45000, today)}}}
	assert.NotEmpty(t, Check(exp, s, wrongAmount, today))

	wrongDate := Outcome{Diff: Diff{Created: []domain.Expense{coffee(35000, today.AddDate(0, 0, -1))}}}
	assert.NotEmpty(t, Check(exp, s, wrongDate, today))

	nothing := Outcome{}
	assert.NotEmpty(t, Check(exp, s, nothing, today))
}

func TestCheckRejectsUnexpectedMutations(t *testing.T) {
	s := seeded(t)
	dup := Outcome{Diff: Diff{Created: []domain.Expense{coffee(35000, today), coffee(35000, today)}}}
	assert.NotEmpty(t, Check(Expect{Created: []ExpenseMatch{{Amount: "350"}}}, s, dup, today), "double add")

	cinema := s["cinema"]
	edited := cinema
	edited.Amount = 1
	sneaky := Outcome{Diff: Diff{Created: []domain.Expense{coffee(35000, today)}, Updated: []Change{{Before: cinema, After: edited}}}}
	assert.NotEmpty(t, Check(Expect{Created: []ExpenseMatch{{Amount: "350"}}}, s, sneaky, today), "extra edit")

	assert.NotEmpty(t, Check(Expect{}, s, Outcome{Diff: Diff{Deleted: []domain.Expense{s["mama_dress"]}}}, today), "no expectation means no mutations")
}

func TestCheckUpdatedAndDeleted(t *testing.T) {
	s := seeded(t)
	before := s["coffee_today"]
	after := before
	after.Amount = 45000

	exp := Expect{Updated: []UpdateMatch{{Key: "coffee_today", Amount: "450"}}}
	assert.Empty(t, Check(exp, s, Outcome{Diff: Diff{Updated: []Change{{Before: before, After: after}}}}, today))

	wrong := after
	wrong.Amount = 40000
	assert.NotEmpty(t, Check(exp, s, Outcome{Diff: Diff{Updated: []Change{{Before: before, After: wrong}}}}, today))

	del := Expect{Deleted: []string{"taxi_yesterday"}}
	assert.Empty(t, Check(del, s, Outcome{Diff: Diff{Deleted: []domain.Expense{s["taxi_yesterday"]}}}, today))
	assert.NotEmpty(t, Check(del, s, Outcome{Diff: Diff{Deleted: []domain.Expense{s["taxi_sep"]}}}, today))
}

func TestCheckUpdatedRejectsSideEffects(t *testing.T) {
	s := seeded(t)
	before := s["coffee_today"]
	exp := Expect{Updated: []UpdateMatch{{Key: "coffee_today", Amount: "450"}}}
	run := func(exp Expect, mutate func(*domain.Expense)) []string {
		after := before
		after.Amount = 45000
		mutate(&after)
		return Check(exp, s, Outcome{Diff: Diff{Updated: []Change{{Before: before, After: after}}}}, today)
	}

	assert.Empty(t, run(exp, func(*domain.Expense) {}))
	assert.NotEmpty(t, run(exp, func(e *domain.Expense) { e.Category = domain.CategoryOther }), "category also changed")
	assert.NotEmpty(t, run(exp, func(e *domain.Expense) { e.Comment = "другое" }), "comment also changed")
	assert.NotEmpty(t, run(exp, func(e *domain.Expense) { e.SpentAt = e.SpentAt.AddDate(0, 0, -1) }), "date also changed")
	assert.NotEmpty(t, run(exp, func(e *domain.Expense) { e.UserID = 222 }), "owner changed")

	commentExp := Expect{Updated: []UpdateMatch{{Key: "coffee_today", Amount: "450", Comment: "капучино"}}}
	assert.Empty(t, run(commentExp, func(e *domain.Expense) { e.Comment = "Капучино" }), "expected comment change")
	assert.NotEmpty(t, run(commentExp, func(*domain.Expense) {}), "expected comment not set")

	noop := Expect{Updated: []UpdateMatch{{Key: "coffee_today"}}}
	assert.NotEmpty(t, run(noop, func(*domain.Expense) {}), "empty UpdateMatch no longer accepts any change")
}

func TestCheckNormalizesReply(t *testing.T) {
	s := seeded(t)
	exp := Expect{ReplyContains: []string{"4 400"}, ReplyNotContains: []string{"платье"}, Ask: false}
	for _, reply := range []string{"Еда: 4 400 ₽", "Еда: 4\u00a0400\u00a0₽", "ЕДА: 4\u202f400 ₽"} {
		assert.Empty(t, Check(exp, s, Outcome{Reply: reply}, today), reply)
	}
	assert.NotEmpty(t, Check(exp, s, Outcome{Reply: "4 400 ₽, а мама купила Платье"}, today))
	assert.NotEmpty(t, Check(Expect{Ask: true}, s, Outcome{Reply: "Удалил"}, today))
	assert.Empty(t, Check(Expect{Ask: true}, s, Outcome{Reply: "Какую именно?"}, today))
	assert.Empty(t, Check(Expect{ReplyContains: []string{"жилье"}}, s, Outcome{Reply: "Больше всего — жильё"}, today))
}

func TestCheckTools(t *testing.T) {
	s := seeded(t)
	exp := Expect{Tools: []string{"get_summary"}}
	assert.Empty(t, Check(exp, s, Outcome{Tools: []string{"find_expenses", "get_summary"}}, today))
	assert.NotEmpty(t, Check(exp, s, Outcome{Tools: []string{"find_expenses"}}, today))
}

func TestResolveDate(t *testing.T) {
	d, err := ResolveDate("today-1", today)
	require.NoError(t, err)
	assert.Equal(t, "2026-10-14", domain.FormatDate(d))
	d, err = ResolveDate("2026-10-12", today)
	require.NoError(t, err)
	assert.Equal(t, "2026-10-12", domain.FormatDate(d))
	_, err = ResolveDate("yesterday", today)
	assert.Error(t, err)
}

func TestCheckFlagsConfirmationRepeatedByModel(t *testing.T) {
	s := seeded(t)
	coffee := domain.Expense{ID: 999, Amount: 35000, Category: domain.CategoryFood, SpentAt: today, Comment: "кофе"}
	exp := Expect{Created: []ExpenseMatch{{Amount: "350"}}}
	out := Outcome{Diff: Diff{Created: []domain.Expense{coffee}}}

	out.Reply = "Записал: кофе — 350 ₽, еда, 15 октября"
	assert.Empty(t, Check(exp, s, out, today))
	out.Reply = "Записал: кофе — 350 ₽, еда, 15 октября\n\nЗа неделю — 1 550 ₽."
	assert.Empty(t, Check(exp, s, out, today))
	out.Reply = "Записал: кофе — 350 ₽, еда, 15 октября\n\nЗаписал кофе за 350 ₽"
	assert.NotEmpty(t, Check(exp, s, out, today), "the model restated the confirmation")
	assert.Empty(t, Check(Expect{}, s, Outcome{Reply: "Ничего не удалил: уточни, какую трату"}, today))
}
