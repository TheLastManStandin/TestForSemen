package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

func moscow(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	return loc
}

func TestResolvePeriod(t *testing.T) {
	// Thursday 2026-10-15, 12:00 Moscow
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, moscow(t))
	cases := []struct {
		p        domain.Period
		from, to string
	}{
		{domain.PeriodToday, "2026-10-15", "2026-10-15"},
		{domain.PeriodYesterday, "2026-10-14", "2026-10-14"},
		{domain.PeriodThisWeek, "2026-10-12", "2026-10-15"},
		{domain.PeriodLastWeek, "2026-10-05", "2026-10-11"},
		{domain.PeriodThisMonth, "2026-10-01", "2026-10-15"},
		{domain.PeriodLastMonth, "2026-09-01", "2026-09-30"},
		{domain.PeriodLast7Days, "2026-10-09", "2026-10-15"},
		{domain.PeriodLast30Days, "2026-09-16", "2026-10-15"},
	}
	for _, c := range cases {
		t.Run(string(c.p), func(t *testing.T) {
			from, to, err := domain.ResolvePeriod(c.p, now)
			require.NoError(t, err)
			assert.Equal(t, c.from, domain.FormatDate(from))
			assert.Equal(t, c.to, domain.FormatDate(to))
		})
	}
}

func TestResolvePeriodWeekEdges(t *testing.T) {
	loc := moscow(t)
	sunday := time.Date(2026, 10, 18, 10, 0, 0, 0, loc)
	from, to, err := domain.ResolvePeriod(domain.PeriodThisWeek, sunday)
	require.NoError(t, err)
	assert.Equal(t, "2026-10-12", domain.FormatDate(from))
	assert.Equal(t, "2026-10-18", domain.FormatDate(to))

	monday := time.Date(2026, 10, 12, 10, 0, 0, 0, loc)
	from, to, err = domain.ResolvePeriod(domain.PeriodThisWeek, monday)
	require.NoError(t, err)
	assert.Equal(t, "2026-10-12", domain.FormatDate(from))
	assert.Equal(t, "2026-10-12", domain.FormatDate(to))
}

func TestResolvePeriodLastMonthInJanuary(t *testing.T) {
	now := time.Date(2026, 1, 10, 10, 0, 0, 0, moscow(t))
	from, to, err := domain.ResolvePeriod(domain.PeriodLastMonth, now)
	require.NoError(t, err)
	assert.Equal(t, "2025-12-01", domain.FormatDate(from))
	assert.Equal(t, "2025-12-31", domain.FormatDate(to))
}

func TestResolvePeriodUnknown(t *testing.T) {
	_, _, err := domain.ResolvePeriod("fortnight", time.Now())
	assert.Error(t, err)
}

func TestDateOfUsesTimeLocation(t *testing.T) {
	// 22:30 UTC is already the next day in Moscow (UTC+3).
	utc := time.Date(2026, 10, 15, 22, 30, 0, 0, time.UTC)
	assert.Equal(t, "2026-10-16", domain.FormatDate(domain.DateOf(utc.In(moscow(t)))))
	assert.Equal(t, time.UTC, domain.DateOf(utc).Location())
}

func TestParseDate(t *testing.T) {
	d, err := domain.ParseDate("2026-10-14")
	require.NoError(t, err)
	assert.Equal(t, "2026-10-14", domain.FormatDate(d))
	assert.Equal(t, "среда", domain.WeekdayRU(d))

	_, err = domain.ParseDate("14.10.2026")
	assert.Error(t, err)
}
