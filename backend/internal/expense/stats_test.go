package expense_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
)

func statsFixture(t *testing.T) *expense.Service {
	t.Helper()
	svc, _ := newService(t, fixedNow)
	mustCreate(t, svc, papa, 100, domain.CategoryFood, "2026-10-14", "кофе")
	mustCreate(t, svc, papa, 400, domain.CategoryTransport, "2026-10-15", "такси")
	mustCreate(t, svc, mama, 200, domain.CategoryFood, "2026-10-15", "обед")
	mustCreate(t, svc, papa, 50, domain.CategoryFood, "2026-10-13", "вчерашний кофе") // previous period
	return svc
}

func TestStats(t *testing.T) {
	svc := statsFixture(t)
	st, err := svc.Stats(ctx, expense.StatsQuery{From: expensetest.Date("2026-10-14"), To: expensetest.Date("2026-10-15")})
	require.NoError(t, err)

	assert.Equal(t, domain.Money(700), st.Total)
	assert.Equal(t, 3, st.Count)
	assert.Equal(t, domain.Money(50), st.PrevTotal, "previous period is 2026-10-12..13")
	assert.Equal(t, []expense.CategoryTotal{
		{Category: domain.CategoryTransport, Total: 400, Count: 1},
		{Category: domain.CategoryFood, Total: 300, Count: 2},
	}, st.ByCategory)
	assert.Equal(t, []expense.MemberTotal{
		{UserID: papa, MemberName: "Папа", Total: 500, Count: 2},
		{UserID: mama, MemberName: "Мама", Total: 200, Count: 1},
	}, st.ByMember)
	require.Len(t, st.ByDay, 3)
	assert.Equal(t, "2026-10-14", domain.FormatDate(st.ByDay[0].Date))
	assert.Equal(t, papa, st.ByDay[1].UserID)
	assert.Equal(t, mama, st.ByDay[2].UserID)
	assert.Equal(t, domain.Money(200), st.ByDay[2].Total)
}

func TestStatsEmptyHasNonNilSlices(t *testing.T) {
	svc, _ := newService(t, fixedNow)
	st, err := svc.Stats(ctx, expense.StatsQuery{From: expensetest.Date("2026-10-01"), To: expensetest.Date("2026-10-15")})
	require.NoError(t, err)
	assert.NotNil(t, st.ByCategory)
	assert.NotNil(t, st.ByMember)
	assert.NotNil(t, st.ByDay)
}

func TestStatsRejectsInvertedRange(t *testing.T) {
	svc, _ := newService(t, fixedNow)
	_, err := svc.Stats(ctx, expense.StatsQuery{From: expensetest.Date("2026-10-15"), To: expensetest.Date("2026-10-14")})
	assert.Equal(t, "to", fieldOf(t, err))
}

func TestSummaryByCategoryIsScoped(t *testing.T) {
	svc := statsFixture(t)
	sum, err := svc.Summary(ctx, expense.Own(papa), expense.SummaryQuery{
		From: expensetest.Date("2026-10-14"), To: expensetest.Date("2026-10-15"), GroupBy: expense.GroupCategory,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.Money(500), sum.Total)
	assert.Equal(t, 2, sum.Count)
	assert.Equal(t, []expense.SummaryGroup{
		{Key: "transport", Label: "транспорт", Total: 400, Count: 1},
		{Key: "food", Label: "еда", Total: 100, Count: 1},
	}, sum.Groups)
}

func TestSummaryByDay(t *testing.T) {
	svc := statsFixture(t)
	sum, err := svc.Summary(ctx, expense.Own(papa), expense.SummaryQuery{
		From: expensetest.Date("2026-10-13"), To: expensetest.Date("2026-10-15"), GroupBy: expense.GroupDay,
	})
	require.NoError(t, err)
	require.Len(t, sum.Groups, 3)
	assert.Equal(t, "2026-10-13", sum.Groups[0].Key)
	assert.Equal(t, "14.10 (среда)", sum.Groups[1].Label)
}

func TestSummaryNoneAndValidation(t *testing.T) {
	svc := statsFixture(t)
	sum, err := svc.Summary(ctx, expense.Own(papa), expense.SummaryQuery{
		From: expensetest.Date("2026-10-01"), To: expensetest.Date("2026-10-15"), Category: ptr(domain.CategoryFood), GroupBy: expense.GroupNone,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.Money(150), sum.Total)
	assert.Empty(t, sum.Groups)

	_, err = svc.Summary(ctx, expense.Own(papa), expense.SummaryQuery{
		From: expensetest.Date("2026-10-01"), To: expensetest.Date("2026-10-15"), GroupBy: "week",
	})
	assert.Equal(t, "group_by", fieldOf(t, err))
}
