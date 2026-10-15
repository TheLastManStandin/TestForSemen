// Package seed fills an empty database with realistic demo expenses.
package seed

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
)

type template struct {
	comment  string
	category domain.Category
	min, max int // roubles
}

var daily = []template{
	{"кофе", domain.CategoryFood, 150, 450},
	{"продукты", domain.CategoryFood, 600, 4500},
	{"обед", domain.CategoryFood, 350, 900},
	{"доставка еды", domain.CategoryFood, 700, 2500},
	{"такси", domain.CategoryTransport, 300, 1500},
	{"метро", domain.CategoryTransport, 60, 120},
	{"бензин", domain.CategoryTransport, 2000, 4000},
	{"парковка", domain.CategoryTransport, 100, 400},
	{"кино", domain.CategoryEntertainment, 400, 1200},
	{"подписка", domain.CategoryEntertainment, 199, 599},
	{"аптека", domain.CategoryHealth, 200, 2500},
	{"одежда", domain.CategoryShopping, 1500, 9000},
	{"подарок", domain.CategoryShopping, 800, 5000},
	{"бытовая химия", domain.CategoryShopping, 300, 1500},
}

// Demo creates up to two random expenses per member per day for the last `days` days,
// plus monthly rent (5th) and utilities (20th) for the first member. It does nothing
// if any expense already exists.
func Demo(ctx context.Context, svc *expense.Service, members []domain.Member, days int, rnd *rand.Rand) (int, error) {
	page, err := svc.List(ctx, expense.All(), expense.ListQuery{Limit: 1})
	if err != nil {
		return 0, err
	}
	if page.Total > 0 || len(members) == 0 {
		return 0, nil
	}

	n := 0
	add := func(user int64, rub int, cat domain.Category, comment string, spentAtOffset int) error {
		_, err := svc.Create(ctx, expense.CreateInput{
			UserID: user, Amount: domain.Money(rub * 100), Category: cat,
			SpentAt: svc.Today().AddDate(0, 0, -spentAtOffset), Comment: comment,
		})
		if err != nil {
			return fmt.Errorf("seed %q: %w", comment, err)
		}
		n++
		return nil
	}

	for offset := days - 1; offset >= 0; offset-- {
		date := svc.Today().AddDate(0, 0, -offset)
		head := members[0].TelegramID
		if date.Day() == 5 {
			if err := add(head, 45000, domain.CategoryHousing, "аренда", offset); err != nil {
				return n, err
			}
		}
		if date.Day() == 20 {
			if err := add(head, 6500, domain.CategoryHousing, "коммуналка", offset); err != nil {
				return n, err
			}
		}
		for _, m := range members {
			for range rnd.IntN(3) {
				t := daily[rnd.IntN(len(daily))]
				rub := (t.min + rnd.IntN(t.max-t.min+1)) / 10 * 10
				if err := add(m.TelegramID, max(rub, 10), t.category, t.comment, offset); err != nil {
					return n, err
				}
			}
		}
	}
	return n, nil
}
