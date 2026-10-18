package agent_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

const addCoffee = `{"amount":"350","category":"food","comment":"кофе"}`

func TestReplyConfirmsChangesFromToolResults(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{calls(tc("a", "add_expense", addCoffee)), text("")}})

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
	require.NoError(t, err)
	want := "Записал: кофе — 350 ₽, еда, 15 октября"
	assert.Equal(t, want, res.Reply)
	assert.Empty(t, f.mem.Traces()[0].Error)

	h := history(t, f, papa)
	assert.Equal(t, want, h[len(h)-1].Content, "history keeps what the member saw")
}

func TestReplyAppendsModelTextAfterConfirmation(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{
		calls(tc("a", "add_expense", addCoffee)),
		text("  За неделю — 1 550 ₽.  "),
	}})

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350, сколько за неделю?"})
	require.NoError(t, err)
	assert.Equal(t, "Записал: кофе — 350 ₽, еда, 15 октября\n\nЗа неделю — 1 550 ₽.", res.Reply)
}

func TestReplyListsEveryChangeInOrder(t *testing.T) {
	f := newFixture(t, nil)
	taxi := seedTaxi(t, f, papa)
	f.llm.replies = []llm.ChatResponse{
		calls(tc("a", "add_expense", addCoffee), tc("d", "delete_expense", fmt.Sprintf(`{"id":%d}`, taxi.ID))),
		text(""),
	}

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350, такси удали"})
	require.NoError(t, err)
	assert.Equal(t, "Записал: кофе — 350 ₽, еда, 15 октября\nУдалил: такси — 1 200 ₽, транспорт, 14 октября", res.Reply)
}

func TestReplyEmptyAfterFailedToolReportsFailure(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{
		calls(tc("a", "add_expense", `{"amount":"0","category":"food","comment":"кофе"}`)),
		text(""),
	}})

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 0"})
	require.NoError(t, err, "a model misstep is a reply, not an infrastructure error")
	assert.Equal(t, "Не получилось: сумма должна быть больше нуля.", res.Reply)
	assert.Empty(t, res.Changes)
	assert.NotContains(t, res.Reply, "Готово")
	assert.Contains(t, f.mem.Traces()[0].Error, agent.ErrEmptyReply.Error())
}

func TestReplyEmptyAfterRecoveredFailureIsNotAFailure(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{
		calls(tc("a", "add_expense", `{"amount":"много","category":"food"}`)),
		calls(tc("b", "add_expense", addCoffee)),
		text(""),
	}})

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
	require.NoError(t, err)
	assert.Equal(t, "Записал: кофе — 350 ₽, еда, 15 октября", res.Reply)
	assert.Empty(t, f.mem.Traces()[0].Error)
}

func TestReplyEmptyWithoutToolsAsksToRephrase(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{text("   ")}})

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "эээ"})
	require.NoError(t, err)
	assert.Equal(t, agent.ReplyNotUnderstood, res.Reply)
	assert.Contains(t, f.mem.Traces()[0].Error, agent.ErrEmptyReply.Error())
}

func TestToolCallIDRepeatedInOneResponseRunsOnce(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{
		calls(tc("a", "add_expense", addCoffee), tc("a", "add_expense", addCoffee)),
		text(""),
	}})

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
	require.NoError(t, err)
	assert.Len(t, own(t, f.svc, papa), 1)
	assert.Len(t, res.Changes, 1)
	assert.Len(t, res.Steps, 1)

	second := f.llm.requests[1].Messages
	assistant, tool := second[len(second)-2], second[len(second)-1]
	assert.Len(t, assistant.ToolCalls, 1, "the duplicate is dropped so every call has one result")
	assert.Equal(t, "a", tool.ToolCallID)
}

func TestToolCallIDsMayRepeatAcrossResponses(t *testing.T) {
	// Some providers number calls per response ("0", "1", ...), so ids are unique only within one.
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{
		calls(tc("0", "add_expense", addCoffee)),
		calls(tc("0", "add_expense", addCoffee)),
		text(""),
	}})

	_, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "два кофе по 350"})
	require.NoError(t, err)
	assert.Len(t, own(t, f.svc, papa), 2, "identical purchases are legitimate")
}

func TestToolCallsWithoutIDGetDistinctIDs(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{
		calls(tc("", "add_expense", addCoffee), tc("", "add_expense", addCoffee)),
		text(""),
	}})

	_, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "два кофе по 350"})
	require.NoError(t, err)
	assert.Len(t, own(t, f.svc, papa), 2)

	second := f.llm.requests[1].Messages
	assistant := second[len(second)-3]
	require.Len(t, assistant.ToolCalls, 2)
	a, b := assistant.ToolCalls[0].ID, assistant.ToolCalls[1].ID
	assert.NotEmpty(t, a)
	assert.NotEqual(t, a, b)
	assert.Equal(t, a, second[len(second)-2].ToolCallID)
	assert.Equal(t, b, second[len(second)-1].ToolCallID)
}
