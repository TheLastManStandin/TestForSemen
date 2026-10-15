package seed_test

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/memrepo"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/seed"
)

func TestDemo(t *testing.T) {
	ctx := context.Background()
	members := []domain.Member{expensetest.Papa, expensetest.Mama}
	repo := memrepo.New()
	require.NoError(t, repo.UpsertMembers(ctx, members))
	now := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	svc := expense.NewService(repo, nil, time.UTC, func() time.Time { return now })

	n, err := seed.Demo(ctx, svc, members, 60, rand.New(rand.NewPCG(1, 2)))
	require.NoError(t, err)
	assert.Greater(t, n, 60)

	all := repo.Snapshot()
	assert.Len(t, all, n)
	earliest := svc.Today().AddDate(0, 0, -59)
	var rent bool
	usedMembers := map[int64]bool{}
	for _, e := range all {
		assert.False(t, e.SpentAt.Before(earliest), "within the window")
		assert.False(t, e.SpentAt.After(svc.Today()))
		assert.NotEmpty(t, e.Comment)
		usedMembers[e.UserID] = true
		if e.Category == domain.CategoryHousing && e.Comment == "аренда" {
			rent = true
			assert.Equal(t, 5, e.SpentAt.Day())
		}
	}
	assert.True(t, rent, "monthly rent on the 5th")
	assert.Len(t, usedMembers, 2)

	again, err := seed.Demo(ctx, svc, members, 60, rand.New(rand.NewPCG(1, 2)))
	require.NoError(t, err)
	assert.Zero(t, again, "seeding is skipped when data exists")
}
