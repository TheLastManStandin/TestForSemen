package agent_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/memrepo"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

// racingRepo simulates a panel edit of the comment landing just before the agent's update is written.
type racingRepo struct {
	*memrepo.Repo
	comment string
}

func (r *racingRepo) Update(ctx context.Context, id int64, p expense.Patch, owner *int64) (domain.Expense, error) {
	if r.comment != "" {
		c := r.comment
		r.comment = ""
		if _, err := r.Repo.Update(ctx, id, expense.Patch{Comment: &c}, nil); err != nil {
			return domain.Expense{}, err
		}
	}
	return r.Repo.Update(ctx, id, p, owner)
}

func TestPartialUpdateAttributesOnlyPatchedFields(t *testing.T) {
	repo := &racingRepo{Repo: memrepo.New()}
	require.NoError(t, repo.UpsertMembers(ctx, []domain.Member{expensetest.Papa, expensetest.Mama}))
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	clock := func() time.Time { return fixedNow }
	svc := expense.NewService(repo, nil, loc, clock)
	taxi, err := svc.Create(ctx, expense.CreateInput{
		UserID: papa, Amount: 120000, Category: domain.CategoryTransport,
		SpentAt: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), Comment: "такси",
	})
	require.NoError(t, err)
	repo.comment = "такси домой"

	client := &scriptedLLM{
		replies: []llm.ChatResponse{calls(tc("u", "update_expense", fmt.Sprintf(`{"id":%d,"amount":"1300"}`, taxi.ID)))},
		errs:    map[int]error{1: errors.New("boom")},
	}
	mem := agent.NewMemoryStore(clock)
	a := agent.New(client, svc, mem, mem, agent.Config{Model: "test-model"}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	res, err := a.Run(ctx, agent.Input{UserID: papa, Text: "такси было 1300"})
	require.ErrorIs(t, err, agent.ErrPartial)
	assert.Equal(t, partialHeader+"\nИзменил: такси домой — 1 300 ₽, транспорт, 14 октября (было 1 200 ₽)", res.Reply,
		"the concurrent comment edit is not attributed to the bot")
}
