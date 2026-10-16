package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

func TestHumanDate(t *testing.T) {
	today := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "16 октября", domain.HumanDate(today, today))
	assert.Equal(t, "1 марта", domain.HumanDate(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), today))
	assert.Equal(t, "31 декабря 2025", domain.HumanDate(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), today))
}
