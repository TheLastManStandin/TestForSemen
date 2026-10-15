package bot_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/bot"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/stt"
)

const (
	member   int64 = 111
	stranger int64 = 999
	chat     int64 = 5000
)

type fakeMessenger struct {
	mu        sync.Mutex
	sent      []string
	typing    int
	files     map[string][]byte
	downloads int
}

func (f *fakeMessenger) SendText(ctx context.Context, chatID int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if chatID != chat {
		return errors.New("wrong chat")
	}
	f.sent = append(f.sent, text)
	return nil
}

func (f *fakeMessenger) SendTyping(context.Context, int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.typing++
	return nil
}

func (f *fakeMessenger) DownloadFile(_ context.Context, fileID string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.downloads++
	data, ok := f.files[fileID]
	if !ok {
		return nil, errors.New("no such file")
	}
	return data, nil
}

func (f *fakeMessenger) last(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.NotEmpty(t, f.sent)
	return f.sent[len(f.sent)-1]
}

type fakeAgent struct {
	mu     sync.Mutex
	inputs []agent.Input
	reply  string
	err    error
}

func (f *fakeAgent) Run(_ context.Context, in agent.Input) (agent.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inputs = append(f.inputs, in)
	return agent.Result{Reply: f.reply}, f.err
}

type fakeMembers map[int64]bool

func (m fakeMembers) MemberExists(_ context.Context, id int64) (bool, error) { return m[id], nil }

type fakeSTT struct {
	text   string
	err    error
	calls  int
	format string
	audio  []byte
}

func (f *fakeSTT) Transcribe(_ context.Context, audio []byte, format string) (string, error) {
	f.calls++
	f.format = format
	f.audio = audio
	return f.text, f.err
}

type fakeTraces struct {
	mu     sync.Mutex
	traces []domain.Trace
}

func (f *fakeTraces) SaveTrace(_ context.Context, t domain.Trace) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.traces = append(f.traces, t)
	return nil
}

type env struct {
	traces *fakeTraces
	msg    *fakeMessenger
	agent  *fakeAgent
	stt    *fakeSTT
	h      *bot.Handler
}

func newEnv(t *testing.T, withSTT bool) *env {
	t.Helper()
	e := &env{
		msg:    &fakeMessenger{files: map[string][]byte{"voice-1": []byte("OggS")}},
		agent:  &fakeAgent{reply: "Записал: кофе — 350 ₽"},
		stt:    &fakeSTT{text: "кофе триста пятьдесят"},
		traces: &fakeTraces{},
	}
	var tr stt.Transcriber
	if withSTT {
		tr = e.stt
	}
	cfg := bot.Config{MaxVoiceSeconds: 60, TypingInterval: 10 * time.Millisecond, Traces: e.traces, STTModel: "openai/whisper-large-v3"}
	e.h = bot.NewHandler(e.msg, e.agent, fakeMembers{member: true}, tr, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return e
}

func TestNonMemberIsRefusedWithoutLLM(t *testing.T) {
	e := newEnv(t, true)
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: stranger, Text: "кофе 350"})
	assert.Equal(t, "Этот бот семейный. Попросите главу семьи добавить ваш ID: 999", e.msg.last(t))
	assert.Empty(t, e.agent.inputs)
}

func TestStartCommandGreets(t *testing.T) {
	e := newEnv(t, true)
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Text: "/start"})
	assert.Contains(t, e.msg.last(t), "кофе 350")
	assert.Empty(t, e.agent.inputs)
}

func TestTextGoesToAgent(t *testing.T) {
	e := newEnv(t, true)
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Text: "  кофе 350  "})
	require.Len(t, e.agent.inputs, 1)
	in := e.agent.inputs[0]
	assert.Equal(t, member, in.UserID)
	assert.Equal(t, "кофе 350", in.Text)
	assert.Equal(t, agent.SourceText, in.Source)
	assert.Nil(t, in.STTms)
	assert.Equal(t, "Записал: кофе — 350 ₽", e.msg.last(t))
	assert.GreaterOrEqual(t, e.msg.typing, 1)
}

func TestAgentErrorIsPolite(t *testing.T) {
	e := newEnv(t, true)
	e.agent.err = errors.New("llm down")
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Text: "кофе 350"})
	assert.Equal(t, "Что-то пошло не так, попробуй ещё раз через минуту", e.msg.last(t))
}

func TestVoiceIsTranscribedAndEchoed(t *testing.T) {
	e := newEnv(t, true)
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Voice: &bot.Voice{FileID: "voice-1", Duration: 3}})
	assert.Equal(t, "ogg", e.stt.format)
	assert.Equal(t, []byte("OggS"), e.stt.audio)
	require.Len(t, e.agent.inputs, 1)
	in := e.agent.inputs[0]
	assert.Equal(t, agent.SourceVoice, in.Source)
	assert.Equal(t, "кофе триста пятьдесят", in.Text)
	require.NotNil(t, in.STTms)
	assert.Equal(t, "🎤 «кофе триста пятьдесят»\n\nЗаписал: кофе — 350 ₽", e.msg.last(t))
}

func TestVoiceTooLongIsRejectedBeforeDownload(t *testing.T) {
	e := newEnv(t, true)
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Voice: &bot.Voice{FileID: "voice-1", Duration: 61}})
	assert.Contains(t, e.msg.last(t), "60 секунд")
	assert.Zero(t, e.msg.downloads)
	assert.Zero(t, e.stt.calls)
}

func TestVoiceDisabled(t *testing.T) {
	e := newEnv(t, false)
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Voice: &bot.Voice{FileID: "voice-1", Duration: 3}})
	assert.Contains(t, e.msg.last(t), "выключены")
	assert.Empty(t, e.agent.inputs)
}

func TestVoiceTranscriptionFailure(t *testing.T) {
	for name, s := range map[string]*fakeSTT{
		"error": {err: errors.New("stt down")},
		"empty": {text: "   "},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, true)
			*e.stt = *s
			e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Voice: &bot.Voice{FileID: "voice-1", Duration: 3}})
			assert.Equal(t, "Не разобрал голосовое 🙏 Попробуй ещё раз или напиши текстом", e.msg.last(t))
			assert.NotContains(t, e.msg.last(t), "stt down")
			assert.Empty(t, e.agent.inputs)
		})
	}
}

func TestUnsupportedMessage(t *testing.T) {
	e := newEnv(t, true)
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member})
	assert.Equal(t, "Я понимаю текст и голосовые 🙂", e.msg.last(t))
	assert.Empty(t, e.agent.inputs)
}

func TestLongReplyIsTruncated(t *testing.T) {
	e := newEnv(t, true)
	e.agent.reply = strings.Repeat("я", 5000)
	e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Text: "покажи всё"})
	reply := e.msg.last(t)
	assert.LessOrEqual(t, utf8.RuneCountInString(reply), 4096)
	assert.True(t, strings.HasSuffix(reply, "…"))
}

func TestVoiceFailuresAreTraced(t *testing.T) {
	t.Run("stt error", func(t *testing.T) {
		e := newEnv(t, true)
		*e.stt = fakeSTT{err: errors.New("stt down")}
		e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Voice: &bot.Voice{FileID: "voice-1", Duration: 3}})
		require.Len(t, e.traces.traces, 1)
		tr := e.traces.traces[0]
		assert.Equal(t, "voice", tr.Source)
		assert.Equal(t, member, tr.UserID)
		assert.Contains(t, tr.Error, "stt down")
		assert.Equal(t, "openai/whisper-large-v3", tr.Model)
		assert.NotNil(t, tr.STTms)
	})
	t.Run("empty transcript", func(t *testing.T) {
		e := newEnv(t, true)
		*e.stt = fakeSTT{text: "  "}
		e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Voice: &bot.Voice{FileID: "voice-1", Duration: 3}})
		require.Len(t, e.traces.traces, 1)
		assert.Equal(t, "empty transcript", e.traces.traces[0].Error)
		assert.NotNil(t, e.traces.traces[0].STTms)
	})
	t.Run("download error", func(t *testing.T) {
		e := newEnv(t, true)
		e.h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Voice: &bot.Voice{FileID: "missing", Duration: 3}})
		require.Len(t, e.traces.traces, 1)
		tr := e.traces.traces[0]
		assert.Equal(t, "voice", tr.Source)
		assert.NotEmpty(t, tr.Error)
		assert.Equal(t, "openai/whisper-large-v3", tr.Model)
		assert.Equal(t, "Не разобрал голосовое 🙏 Попробуй ещё раз или напиши текстом", e.msg.last(t))
	})
	t.Run("nil sink is fine", func(t *testing.T) {
		e := newEnv(t, true)
		h := bot.NewHandler(e.msg, e.agent, fakeMembers{member: true}, &fakeSTT{err: errors.New("x")}, bot.Config{}, nil)
		h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Voice: &bot.Voice{FileID: "voice-1", Duration: 3}})
		assert.Contains(t, e.msg.last(t), "Не разобрал")
	})
}

// blockingAgent waits for the message ctx to expire, like an LLM that never answers.
type blockingAgent struct{}

func (blockingAgent) Run(ctx context.Context, _ agent.Input) (agent.Result, error) {
	<-ctx.Done()
	return agent.Result{}, context.DeadlineExceeded
}

func TestInternalErrorIsSentAfterMessageTimeout(t *testing.T) {
	e := newEnv(t, true)
	h := bot.NewHandler(e.msg, blockingAgent{}, fakeMembers{member: true}, e.stt, bot.Config{MessageTimeout: 20 * time.Millisecond}, nil)
	h.Handle(t.Context(), bot.Incoming{ChatID: chat, UserID: member, Text: "кофе 200"})
	assert.Equal(t, "Что-то пошло не так, попробуй ещё раз через минуту", e.msg.last(t))
}
