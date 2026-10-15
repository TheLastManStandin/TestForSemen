package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Money is an amount in kopecks (1/100 of a rouble).
type Money int64

var errBadAmount = errors.New("некорректная сумма")

var separatorStripper = strings.NewReplacer(" ", "", " ", "", " ", "", "_", "")

// ParseMoney parses amounts like "350", "350.5", "350,50", "1 200", "1 200,50 ₽".
func ParseMoney(s string) (Money, error) {
	clean := strings.TrimSpace(s)
	clean = strings.TrimSpace(strings.TrimSuffix(clean, "₽"))
	clean = separatorStripper.Replace(clean)
	clean = strings.Replace(clean, ",", ".", 1)
	if clean == "" {
		return 0, fmt.Errorf("%w: пустое значение", errBadAmount)
	}

	whole, frac, hasFrac := strings.Cut(clean, ".")
	if whole == "" {
		whole = "0"
	}
	if !isDigits(whole) || len(whole) > 15 {
		return 0, fmt.Errorf("%w: %q", errBadAmount, s)
	}
	switch {
	case !hasFrac:
		frac = "00"
	case len(frac) == 1 && isDigits(frac):
		frac += "0"
	case len(frac) == 2 && isDigits(frac):
	default:
		return 0, fmt.Errorf("%w: %q", errBadAmount, s)
	}

	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", errBadAmount, s)
	}
	f, _ := strconv.ParseInt(frac, 10, 64)
	return Money(w*100 + f), nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// String renders the API representation, e.g. "1200.50".
func (m Money) String() string {
	return fmt.Sprintf("%d.%02d", int64(m)/100, int64(m)%100)
}

// Human renders the user-facing representation, e.g. "1 200 ₽" or "1 200,50 ₽".
func (m Money) Human() string {
	whole := groupThousands(int64(m) / 100)
	if kop := int64(m) % 100; kop != 0 {
		return fmt.Sprintf("%s,%02d ₽", whole, kop)
	}
	return whole + " ₽"
}

func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}
