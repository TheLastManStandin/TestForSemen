package bot_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/bot"
)

const partialSummary = "Не удалось закончить ответ, но действия выполнены:\n• записал: кофе — 350 ₽, еда, 15 октября"

func TestPartialAgentFailureSendsSummary(t *testing.T) {
	e := newEnv(t, true)
	e.agent.reply = partialSummary
	e.agent.err = fmt.Errorf("%w: llm: %w", agent.ErrPartial, errors.New("boom"))
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, ChatType: "private", UserID: member, Text: "кофе 350"})
	assert.Equal(t, partialSummary, e.msg.last(t))
}

func TestPartialAgentFailureKeepsVoiceEcho(t *testing.T) {
	e := newEnv(t, true)
	e.agent.reply = partialSummary
	e.agent.err = fmt.Errorf("%w: llm: %w", agent.ErrPartial, errors.New("boom"))
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, ChatType: "private", UserID: member, Voice: &bot.Voice{FileID: "voice-1", Duration: 3}})
	assert.Equal(t, "🎤 «кофе триста пятьдесят»\n\n"+partialSummary, e.msg.last(t))
}

func TestPartialErrorWithoutSummaryFallsBackToGenericText(t *testing.T) {
	e := newEnv(t, true)
	e.agent.reply = ""
	e.agent.err = agent.ErrPartial
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, ChatType: "private", UserID: member, Text: "кофе 350"})
	assert.Equal(t, "Что-то пошло не так, попробуй ещё раз через минуту", e.msg.last(t))
}
