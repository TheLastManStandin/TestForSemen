package domain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

func TestParseCategory(t *testing.T) {
	cases := []struct {
		in   string
		want domain.Category
	}{
		{"food", domain.CategoryFood},
		{" Transport", domain.CategoryTransport},
		{"еда", domain.CategoryFood},
		{"Жильё", domain.CategoryHousing},
		{"жилье", domain.CategoryHousing},
		{"другое", domain.CategoryOther},
	}
	for _, c := range cases {
		got, err := domain.ParseCategory(c.in)
		require.NoError(t, err, c.in)
		assert.Equal(t, c.want, got, c.in)
	}
}

func TestParseCategoryRejectsUnknown(t *testing.T) {
	_, err := domain.ParseCategory("pets")
	var ve *domain.ValidationError
	require.True(t, errors.As(err, &ve))
	assert.Equal(t, "category", ve.Field)
}

func TestCategoryRU(t *testing.T) {
	assert.Equal(t, "развлечения", domain.CategoryEntertainment.RU())
	assert.Len(t, domain.Categories, 7)
	for _, c := range domain.Categories {
		assert.True(t, c.Valid(), c)
	}
	assert.False(t, domain.Category("pets").Valid())
}
