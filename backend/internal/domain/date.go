package domain

import (
	"fmt"
	"strings"
	"time"
)

// DateLayout is the ISO date format used in the API and agent tools.
const DateLayout = "2006-01-02"

// DateOf returns midnight UTC of t's calendar day in t's own location.
// Convert t to the user's location first: DateOf(now.In(loc)).
func DateOf(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ParseDate parses "YYYY-MM-DD" into a domain date.
func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse(DateLayout, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, Invalid("date", fmt.Sprintf("дата %q не в формате ГГГГ-ММ-ДД", s))
	}
	return t, nil
}

// FormatDate renders a domain date as "YYYY-MM-DD".
func FormatDate(t time.Time) string { return t.Format(DateLayout) }

var weekdaysRU = [...]string{"воскресенье", "понедельник", "вторник", "среда", "четверг", "пятница", "суббота"}

// WeekdayRU returns the Russian weekday name.
func WeekdayRU(t time.Time) string { return weekdaysRU[t.Weekday()] }
