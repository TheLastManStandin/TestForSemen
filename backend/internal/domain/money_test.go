package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

func TestParseMoney(t *testing.T) {
	cases := []struct {
		in   string
		want domain.Money
	}{
		{"350", 35000},
		{"350.5", 35050},
		{"350,50", 35050},
		{"350.00", 35000},
		{"1 200", 120000},
		{"1 200,50 ₽", 120050},
		{"1 200", 120000},
		{"0.99", 99},
		{".5", 50},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := domain.ParseMoney(c.in)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestParseMoneyRejects(t *testing.T) {
	for _, in := range []string{"", " ", "₽", "abc", "-5", "1.234", "12.3.4", "1e3", "1234567890123456"} {
		_, err := domain.ParseMoney(in)
		assert.Error(t, err, "input %q", in)
	}
}

func TestMoneyString(t *testing.T) {
	assert.Equal(t, "350.00", domain.Money(35000).String())
	assert.Equal(t, "350.50", domain.Money(35050).String())
	assert.Equal(t, "0.99", domain.Money(99).String())
	assert.Equal(t, "0.00", domain.Money(0).String())
}

func TestMoneyHuman(t *testing.T) {
	assert.Equal(t, "350 ₽", domain.Money(35000).Human())
	assert.Equal(t, "1 200 ₽", domain.Money(120000).Human())
	assert.Equal(t, "1 200,50 ₽", domain.Money(120050).Human())
	assert.Equal(t, "1 234 567,89 ₽", domain.Money(123456789).Human())
	assert.Equal(t, "0,99 ₽", domain.Money(99).Human())
}
