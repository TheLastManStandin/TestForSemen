package domain

import (
	"fmt"
	"time"
)

// Period is a named relative date range the agent can ask for.
type Period string

const (
	PeriodToday      Period = "today"
	PeriodYesterday  Period = "yesterday"
	PeriodThisWeek   Period = "this_week"
	PeriodLastWeek   Period = "last_week"
	PeriodThisMonth  Period = "this_month"
	PeriodLastMonth  Period = "last_month"
	PeriodLast7Days  Period = "last_7_days"
	PeriodLast30Days Period = "last_30_days"
)

// Periods lists all supported periods.
var Periods = []Period{
	PeriodToday, PeriodYesterday, PeriodThisWeek, PeriodLastWeek,
	PeriodThisMonth, PeriodLastMonth, PeriodLast7Days, PeriodLast30Days,
}

// ResolvePeriod returns the inclusive [from, to] domain dates of p relative to now.
// now must already be in the user's location. Weeks start on Monday.
func ResolvePeriod(p Period, now time.Time) (from, to time.Time, err error) {
	today := DateOf(now)
	sinceMonday := (int(today.Weekday()) + 6) % 7
	monday := today.AddDate(0, 0, -sinceMonday)
	firstOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)

	switch p {
	case PeriodToday:
		return today, today, nil
	case PeriodYesterday:
		y := today.AddDate(0, 0, -1)
		return y, y, nil
	case PeriodThisWeek:
		return monday, today, nil
	case PeriodLastWeek:
		return monday.AddDate(0, 0, -7), monday.AddDate(0, 0, -1), nil
	case PeriodThisMonth:
		return firstOfMonth, today, nil
	case PeriodLastMonth:
		return firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1), nil
	case PeriodLast7Days:
		return today.AddDate(0, 0, -6), today, nil
	case PeriodLast30Days:
		return today.AddDate(0, 0, -29), today, nil
	}
	return time.Time{}, time.Time{}, Invalid("period", fmt.Sprintf("неизвестный период %q", p))
}
