package evals

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
)

var moscow = mustLoad("Europe/Moscow")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// Now is the frozen eval clock: Thursday 2026-10-15, 12:00 Moscow.
var Now = time.Date(2026, 10, 15, 12, 0, 0, 0, moscow)

// The agent always acts as User; Other's expenses must stay invisible and untouched.
var (
	User    = domain.Member{TelegramID: 111, Name: "Папа"}
	Other   = domain.Member{TelegramID: 222, Name: "Мама"}
	Members = []domain.Member{User, Other}
)

type seedItem struct {
	key      string
	user     int64
	amount   domain.Money
	category domain.Category
	date     string
	comment  string
}

// seedItems are inserted in this order. User's items are chronological, so the most
// recently created one ("последняя трата") is coffee_today.
//
// User totals: October 52 280 ₽ (food 4 400 ₽, transport 1 200 ₽), September 8 450 ₽,
// this week (12–15 Oct) 2 200 ₽, today 350 ₽. Two taxis: 14 Oct and 28 Sep.
var seedItems = []seedItem{
	{"mama_dress", Other.TelegramID, 1_200_000, domain.CategoryShopping, "2026-10-13", "платье"},
	{"mama_coffee", Other.TelegramID, 40_000, domain.CategoryFood, "2026-10-15", "кофе"},
	{"sneakers_sep", User.TelegramID, 750_000, domain.CategoryShopping, "2026-09-20", "кроссовки"},
	{"taxi_sep", User.TelegramID, 95_000, domain.CategoryTransport, "2026-09-28", "такси"},
	{"rent", User.TelegramID, 4_500_000, domain.CategoryHousing, "2026-10-05", "аренда"},
	{"pharmacy", User.TelegramID, 78_000, domain.CategoryHealth, "2026-10-08", "аптека"},
	{"groceries", User.TelegramID, 340_000, domain.CategoryFood, "2026-10-10", "продукты"},
	{"cinema", User.TelegramID, 90_000, domain.CategoryEntertainment, "2026-10-11", "кино"},
	{"lunch_mon", User.TelegramID, 65_000, domain.CategoryFood, "2026-10-12", "обед"},
	{"taxi_yesterday", User.TelegramID, 120_000, domain.CategoryTransport, "2026-10-14", "такси"},
	{"coffee_today", User.TelegramID, 35_000, domain.CategoryFood, "2026-10-15", "кофе"},
}

// SeedKeys lists fixture keys usable in cases.
func SeedKeys() []string {
	keys := make([]string, 0, len(seedItems))
	for _, it := range seedItems {
		keys = append(keys, it.key)
	}
	return keys
}

// Seed inserts the fixture and returns the created expenses by key.
func Seed(ctx context.Context, svc *expense.Service) (map[string]domain.Expense, error) {
	out := make(map[string]domain.Expense, len(seedItems))
	for _, it := range seedItems {
		date, err := domain.ParseDate(it.date)
		if err != nil {
			return nil, err
		}
		e, err := svc.Create(ctx, expense.CreateInput{UserID: it.user, Amount: it.amount, Category: it.category, SpentAt: date, Comment: it.comment})
		if err != nil {
			return nil, fmt.Errorf("seed %s: %w", it.key, err)
		}
		out[it.key] = e
	}
	return out, nil
}

// ResolveDate understands "today", "today-N" and YYYY-MM-DD.
func ResolveDate(s string, today time.Time) (time.Time, error) {
	switch {
	case s == "today":
		return today, nil
	case strings.HasPrefix(s, "today-"):
		n, err := strconv.Atoi(strings.TrimPrefix(s, "today-"))
		if err != nil {
			return time.Time{}, fmt.Errorf("bad relative date %q", s)
		}
		return today.AddDate(0, 0, -n), nil
	default:
		return domain.ParseDate(s)
	}
}
