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
	assert.Empty(t, h[len(h)-1].Content, "history keeps the model's own words, so it does not learn to repeat confirmations")
	assert.Equal(t, want, f.mem.Traces()[0].Output)
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

func TestReplyEmptyWithoutChangesAsksToRephrase(t *testing.T) {
	for name, replies := range map[string][]llm.ChatResponse{
		"no tools":     {text("   ")},
		"after a read": {calls(tc("f", "find_expenses", `{}`)), text("")},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, &scriptedLLM{replies: replies})
			res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "что там"})
			require.NoError(t, err)
			assert.Equal(t, agent.ReplyNoAnswer, res.Reply)
			assert.Contains(t, f.mem.Traces()[0].Error, agent.ErrEmptyReply.Error())
		})
	}
}

func TestReplyEmptyReportsEveryFailureOfTheLastBatch(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{
		calls(
			tc("a", "add_expense", `{"amount":"0","category":"food","comment":"кофе"}`),
			tc("d", "delete_expense", `{"id":999999}`),
			tc("b", "add_expense", `{"amount":"0","category":"food","comment":"чай"}`),
		),
		text(""),
	}})

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе и чай по нулям, удали 999999"})
	require.NoError(t, err)
	assert.Equal(t, "Не получилось: сумма должна быть больше нуля; трата не найдена.", res.Reply)
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

func TestToolCallIDSharedByDifferentCallsRunsEach(t *testing.T) {
	// Some providers give every parallel call the same id.
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{
		calls(tc("call_0", "add_expense", addCoffee), tc("call_0", "add_expense", `{"amount":"1200","category":"transport","comment":"такси"}`)),
		text(""),
	}})

	_, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350 и такси 1200"})
	require.NoError(t, err)
	assert.Len(t, own(t, f.svc, papa), 2)

	second := f.llm.requests[1].Messages
	assistant := second[len(second)-3]
	require.Len(t, assistant.ToolCalls, 2)
	a, b := assistant.ToolCalls[0].ID, assistant.ToolCalls[1].ID
	assert.NotEqual(t, a, b, "colliding ids are renamed so each call keeps its own result")
	assert.Equal(t, a, second[len(second)-2].ToolCallID)
	assert.Equal(t, b, second[len(second)-1].ToolCallID)
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

func TestReplyDropsConfirmationsRestatedByModel(t *testing.T) {
	for _, c := range []struct{ model, want string }{
		{"Записал: кофе 350 ₽, 15 октября, еда.", ""},
		{"Записал обе траты.", ""},
		{"Готово! Записал кофе.\nЗа неделю — 1 550 ₽.", "За неделю — 1 550 ₽."},
		{"Добавил кофе. Хочешь посмотреть итоги недели?", "Хочешь посмотреть итоги недели?"},
		{"За неделю — 1 550 ₽.", "За неделю — 1 550 ₽."},
	} {
		t.Run(c.model, func(t *testing.T) {
			f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{calls(tc("a", "add_expense", addCoffee)), text(c.model)}})
			res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
			require.NoError(t, err)
			want := "Записал: кофе — 350 ₽, еда, 15 октября"
			if c.want != "" {
				want += "\n\n" + c.want
			}
			assert.Equal(t, want, res.Reply)
		})
	}
}

func TestReplyKeepsModelConfirmationWordsWithoutChanges(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{text("Удалил бы, но таких трат две. Какую именно?")}})
	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "удали такси"})
	require.NoError(t, err)
	assert.Equal(t, "Удалил бы, но таких трат две. Какую именно?", res.Reply)
}
