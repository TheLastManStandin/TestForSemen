package agent_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

const partialHeader = "Не удалось закончить ответ, но действия выполнены:"

func history(t *testing.T, f *fixture, user int64) []llm.Message {
	t.Helper()
	raw, err := f.mem.RecentChat(ctx, user, fixedNow.Add(-time.Hour), 100)
	require.NoError(t, err)
	msgs := make([]llm.Message, 0, len(raw))
	for _, r := range raw {
		var m llm.Message
		require.NoError(t, json.Unmarshal(r, &m))
		msgs = append(msgs, m)
	}
	return msgs
}

func roles(msgs []llm.Message) []llm.Role {
	out := make([]llm.Role, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Role)
	}
	return out
}

func seedTaxi(t *testing.T, f *fixture, user int64) domain.Expense {
	t.Helper()
	e, err := f.svc.Create(ctx, expense.CreateInput{
		UserID: user, Amount: 120000, Category: domain.CategoryTransport,
		SpentAt: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), Comment: "такси",
	})
	require.NoError(t, err)
	return e
}

func TestPartialAddThenLLMFails(t *testing.T) {
	f := newFixture(t, &scriptedLLM{
		replies: []llm.ChatResponse{calls(tc("c1", "add_expense", `{"amount":"350","category":"food","comment":"кофе"}`))},
		errs:    map[int]error{1: errors.New("boom")},
	})
	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
	require.Error(t, err)
	assert.ErrorIs(t, err, agent.ErrPartial)
	assert.Contains(t, err.Error(), "boom", "the real cause is kept")

	want := partialHeader + "\n• записал: кофе — 350 ₽, еда, 15 октября"
	assert.Equal(t, want, res.Reply)
	require.Len(t, res.Changes, 1)
	assert.Equal(t, agent.ChangeAdd, res.Changes[0].Kind)
	assert.Equal(t, "кофе", res.Changes[0].Expense.Comment)
	assert.Nil(t, res.Changes[0].Before)
	assert.Len(t, own(t, f.svc, papa), 1, "exactly one expense saved")

	h := history(t, f, papa)
	assert.Equal(t, []llm.Role{llm.RoleUser, llm.RoleAssistant, llm.RoleTool, llm.RoleAssistant}, roles(h))
	assert.Equal(t, want, h[len(h)-1].Content, "history closes with the factual summary")

	tr := f.mem.Traces()[0]
	assert.Contains(t, tr.Error, "boom")
	assert.Equal(t, want, tr.Output)
}

func TestPartialUpdateThenLLMFails(t *testing.T) {
	f := newFixture(t, nil)
	taxi := seedTaxi(t, f, papa)
	f.llm.replies = []llm.ChatResponse{calls(tc("u", "update_expense", fmt.Sprintf(`{"id":%d,"amount":"1300"}`, taxi.ID)))}
	f.llm.errs = map[int]error{1: errors.New("boom")}

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "такси было 1300"})
	require.ErrorIs(t, err, agent.ErrPartial)
	assert.Equal(t, partialHeader+"\n• изменил: такси — 1 300 ₽, транспорт, 14 октября (было 1 200 ₽)", res.Reply)
	require.Len(t, res.Changes, 1)
	assert.Equal(t, agent.ChangeUpdate, res.Changes[0].Kind)
	require.NotNil(t, res.Changes[0].Before)
	assert.Equal(t, domain.Money(120000), res.Changes[0].Before.Amount)
	assert.Equal(t, domain.Money(130000), res.Changes[0].Expense.Amount)
}

func TestPartialUpdateListsEveryChangedField(t *testing.T) {
	f := newFixture(t, nil)
	taxi := seedTaxi(t, f, papa)
	f.llm.replies = []llm.ChatResponse{calls(tc("u", "update_expense",
		fmt.Sprintf(`{"id":%d,"comment":"такси домой","category":"other","date":"2026-10-13"}`, taxi.ID)))}
	f.llm.errs = map[int]error{1: errors.New("boom")}

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "поправь такси"})
	require.ErrorIs(t, err, agent.ErrPartial)
	assert.Equal(t, partialHeader+"\n• изменил: такси домой — 1 200 ₽, другое, 13 октября (было такси, транспорт, 14 октября)", res.Reply)
}

func TestPartialDeleteThenLLMFails(t *testing.T) {
	f := newFixture(t, nil)
	taxi := seedTaxi(t, f, papa)
	f.llm.replies = []llm.ChatResponse{calls(tc("d", "delete_expense", fmt.Sprintf(`{"id":%d}`, taxi.ID)))}
	f.llm.errs = map[int]error{1: errors.New("boom")}

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "удали такси"})
	require.ErrorIs(t, err, agent.ErrPartial)
	assert.Equal(t, partialHeader+"\n• удалил: такси — 1 200 ₽, транспорт, 14 октября", res.Reply)
	assert.Empty(t, own(t, f.svc, papa))
	require.Len(t, res.Changes, 1)
	assert.Equal(t, agent.ChangeDelete, res.Changes[0].Kind)
}

func TestPartialIdenticalAddsAreAllReported(t *testing.T) {
	add := `{"amount":"350","category":"food","comment":"кофе"}`
	f := newFixture(t, &scriptedLLM{
		replies: []llm.ChatResponse{calls(tc("a", "add_expense", add), tc("b", "add_expense", add))},
		errs:    map[int]error{1: errors.New("boom")},
	})
	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "два кофе по 350"})
	require.ErrorIs(t, err, agent.ErrPartial)
	line := "\n• записал: кофе — 350 ₽, еда, 15 октября"
	assert.Equal(t, partialHeader+line+line, res.Reply)
	assert.Len(t, own(t, f.svc, papa), 2)
}

func assertNoSaveClaim(t *testing.T, res agent.Result, err error) {
	t.Helper()
	require.Error(t, err)
	assert.NotErrorIs(t, err, agent.ErrPartial)
	assert.Empty(t, res.Changes)
	for _, word := range []string{"записал", "изменил", "удалил", "выполнены", "сохран"} {
		assert.NotContains(t, res.Reply, word)
	}
}

func TestLLMFailsWithoutChanges(t *testing.T) {
	t.Run("before any tool", func(t *testing.T) {
		f := newFixture(t, &scriptedLLM{errs: map[int]error{0: errors.New("boom")}})
		res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
		assertNoSaveClaim(t, res, err)
		assert.Empty(t, history(t, f, papa))
	})
	t.Run("after read-only tools", func(t *testing.T) {
		f := newFixture(t, &scriptedLLM{
			replies: []llm.ChatResponse{calls(tc("f", "find_expenses", `{}`), tc("s", "get_summary", `{}`))},
			errs:    map[int]error{1: errors.New("boom")},
		})
		res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "сколько потратил"})
		assertNoSaveClaim(t, res, err)
		for _, m := range history(t, f, papa) {
			assert.NotContains(t, m.Content, "сохран")
		}
		assert.Contains(t, f.mem.Traces()[0].Error, "boom")
	})
	t.Run("after failed changes", func(t *testing.T) {
		f := newFixture(t, nil)
		mamas := seedTaxi(t, f, mama)
		f.llm.replies = []llm.ChatResponse{calls(
			tc("u", "update_expense", fmt.Sprintf(`{"id":%d,"amount":"1"}`, mamas.ID)),
			tc("d", "delete_expense", `{"id":999999}`),
			tc("a", "add_expense", `{"amount":"много","category":"food","comment":"кофе"}`),
		)}
		f.llm.errs = map[int]error{1: errors.New("boom")}

		res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "поменяй"})
		assertNoSaveClaim(t, res, err)
		got, gErr := f.svc.Get(ctx, expense.All(), mamas.ID)
		require.NoError(t, gErr)
		assert.Equal(t, mamas.Amount, got.Amount)
		assert.Empty(t, own(t, f.svc, papa))
	})
}

func TestStepLimitAfterAddReportsSummary(t *testing.T) {
	replies := []llm.ChatResponse{calls(tc("a", "add_expense", `{"amount":"350","category":"food","comment":"кофе"}`))}
	for i := range 5 {
		replies = append(replies, calls(tc(fmt.Sprint(i), "find_expenses", `{}`)))
	}
	f := newFixture(t, &scriptedLLM{replies: replies})

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
	require.NoError(t, err, "step limit is a polite reply, not an infrastructure error")
	want := partialHeader + "\n• записал: кофе — 350 ₽, еда, 15 октября"
	assert.Equal(t, want, res.Reply)
	assert.NotContains(t, res.Reply, agent.ReplyTooManySteps)

	tr := f.mem.Traces()[0]
	assert.Contains(t, tr.Error, "too many")
	h := history(t, f, papa)
	assert.Equal(t, llm.RoleUser, h[0].Role)
	assert.Equal(t, want, h[len(h)-1].Content)
}

func TestNextTurnSeesPartialSummary(t *testing.T) {
	f := newFixture(t, &scriptedLLM{
		replies: []llm.ChatResponse{
			calls(tc("c1", "add_expense", `{"amount":"350","category":"food","comment":"кофе"}`)),
			{}, // failed call placeholder
			text("Кофе уже записан"),
		},
		errs: map[int]error{1: errors.New("boom")},
	})
	partial, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
	require.ErrorIs(t, err, agent.ErrPartial)

	_, err = f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
	require.NoError(t, err)

	msgs := f.llm.requests[2].Messages
	assert.Equal(t, []llm.Role{llm.RoleSystem, llm.RoleUser, llm.RoleAssistant, llm.RoleTool, llm.RoleAssistant, llm.RoleUser}, roles(msgs))
	assert.Equal(t, partial.Reply, msgs[4].Content)
	assert.Len(t, own(t, f.svc, papa), 1)
}
