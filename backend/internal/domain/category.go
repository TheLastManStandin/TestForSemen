package domain

import (
	"fmt"
	"strings"
)

// Category is an expense category code stored in the database and API.
type Category string

const (
	CategoryFood          Category = "food"
	CategoryTransport     Category = "transport"
	CategoryHousing       Category = "housing"
	CategoryEntertainment Category = "entertainment"
	CategoryHealth        Category = "health"
	CategoryShopping      Category = "shopping"
	CategoryOther         Category = "other"
)

// Categories lists all categories in display order.
var Categories = []Category{
	CategoryFood, CategoryTransport, CategoryHousing, CategoryEntertainment,
	CategoryHealth, CategoryShopping, CategoryOther,
}

var categoryNames = map[Category]string{
	CategoryFood:          "еда",
	CategoryTransport:     "транспорт",
	CategoryHousing:       "жильё",
	CategoryEntertainment: "развлечения",
	CategoryHealth:        "здоровье",
	CategoryShopping:      "покупки",
	CategoryOther:         "другое",
}

// Valid reports whether c is a known category.
func (c Category) Valid() bool {
	_, ok := categoryNames[c]
	return ok
}

// RU returns the Russian display name.
func (c Category) RU() string {
	if name, ok := categoryNames[c]; ok {
		return name
	}
	return string(c)
}

// ParseCategory accepts a code ("food") or a Russian name ("еда", "жилье").
func ParseCategory(s string) (Category, error) {
	v := normalizeYo(strings.ToLower(strings.TrimSpace(s)))
	for c, name := range categoryNames {
		if v == string(c) || v == normalizeYo(name) {
			return c, nil
		}
	}
	return "", Invalid("category", fmt.Sprintf("неизвестная категория %q", s))
}

func normalizeYo(s string) string { return strings.ReplaceAll(s, "ё", "е") }
