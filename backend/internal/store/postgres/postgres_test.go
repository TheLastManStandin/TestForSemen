package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/testutil/pgtest"
)

func TestRepositoryContract(t *testing.T) {
	expensetest.RunRepositoryContract(t, func(t *testing.T) expense.Repository {
		return postgres.New(pgtest.Pool(t))
	})
}

func TestUpdateConcurrency(t *testing.T) {
	expensetest.RunUpdateConcurrency(t, func(t *testing.T) expense.Repository {
		return postgres.New(pgtest.Pool(t))
	})
}

func newStore(t *testing.T) *postgres.Store {
	t.Helper()
	s := postgres.New(pgtest.Pool(t))
	require.NoError(t, s.UpsertMembers(context.Background(), []domain.Member{expensetest.Papa, expensetest.Mama}))
	return s
}

func TestChatHistory(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	papa := expensetest.Papa.TelegramID

	msgs := []json.RawMessage{
		json.RawMessage(`{"role":"user","content":"кофе 350"}`),
		json.RawMessage(`{"role":"assistant","content":"Записал"}`),
		json.RawMessage(`{"role":"user","content":"такси 1200"}`),
	}
	require.NoError(t, s.AppendChat(ctx, papa, msgs))
	require.NoError(t, s.AppendChat(ctx, expensetest.Mama.TelegramID, []json.RawMessage{json.RawMessage(`{"role":"user","content":"чужое"}`)}))

	got, err := s.RecentChat(ctx, papa, time.Now().Add(-time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.JSONEq(t, string(msgs[0]), string(got[0]))
	assert.JSONEq(t, string(msgs[2]), string(got[2]))

	got, err = s.RecentChat(ctx, papa, time.Now().Add(-time.Hour), 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.JSONEq(t, string(msgs[1]), string(got[0]), "limit keeps the latest messages in chronological order")

	got, err = s.RecentChat(ctx, papa, time.Now().Add(time.Hour), 10)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestTraces(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	stt := 420

	require.NoError(t, s.SaveTrace(ctx, domain.Trace{
		UserID: expensetest.Papa.TelegramID, Source: "voice", Input: "кофе 350", Output: "Записал",
		Steps: []domain.TraceStep{{Tool: "add_expense", Args: json.RawMessage(`{"amount":"350"}`), Result: json.RawMessage(`{"created":{"id":1}}`), Ms: 12}},
		Model: "m", PromptTokens: 100, CompletionTokens: 20, STTms: &stt, LatencyMs: 900,
	}))
	require.NoError(t, s.SaveTrace(ctx, domain.Trace{
		UserID: expensetest.Mama.TelegramID, Source: "text", Input: "привет", Model: "m", LatencyMs: 300, Error: "llm: timeout",
	}))

	all, total, err := s.ListTraces(ctx, nil, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, all, 2)
	assert.Equal(t, "Мама", all[0].MemberName, "newest first")
	assert.Equal(t, "llm: timeout", all[0].Error)
	assert.Empty(t, all[0].Output)
	assert.Nil(t, all[0].STTms)
	assert.Empty(t, all[0].Steps)

	papaID := expensetest.Papa.TelegramID
	papaOnly, total, err := s.ListTraces(ctx, &papaID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, papaOnly, 1)
	tr := papaOnly[0]
	assert.Equal(t, "voice", tr.Source)
	assert.Equal(t, "Записал", tr.Output)
	require.NotNil(t, tr.STTms)
	assert.Equal(t, 420, *tr.STTms)
	require.Len(t, tr.Steps, 1)
	assert.Equal(t, "add_expense", tr.Steps[0].Tool)
	assert.JSONEq(t, `{"amount":"350"}`, string(tr.Steps[0].Args))
	assert.False(t, tr.CreatedAt.IsZero())
}

// Compile-time checks: the Postgres store backs the agent's history and traces.
var (
	_ agent.History   = (*postgres.Store)(nil)
	_ agent.TraceSink = (*postgres.Store)(nil)
)
