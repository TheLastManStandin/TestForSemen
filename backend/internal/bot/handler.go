// Package bot routes Telegram messages from family members to the agent.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/stt"
)

const (
	maxMessageRunes = 4096
	replyTimeout    = 10 * time.Second
	chatTypePrivate = "private"
)

// Messenger is the part of Telegram the handler needs.
type Messenger interface {
	SendText(ctx context.Context, chatID int64, text string) error
	SendTyping(ctx context.Context, chatID int64) error
	DownloadFile(ctx context.Context, fileID string) ([]byte, error)
}

// AgentRunner processes one member message.
type AgentRunner interface {
	Run(ctx context.Context, in agent.Input) (agent.Result, error)
}

// MemberChecker tells whether a Telegram user belongs to the family.
type MemberChecker interface {
	MemberExists(ctx context.Context, telegramID int64) (bool, error)
}

// Incoming is a transport-independent message.
type Incoming struct {
	ChatID   int64
	ChatType string // Telegram chat type: "private", "group", "supergroup", "channel"
	UserID   int64
	Text     string
	Voice    *Voice
}

// Voice is a voice note attachment.
type Voice struct {
	FileID   string
	Duration int // seconds
}

// Config tunes the handler.
type Config struct {
	MaxVoiceSeconds int             // default 60
	TypingInterval  time.Duration   // Telegram shows "typing" for ~5s; default 4s
	MessageTimeout  time.Duration   // whole-message budget; default 2m
	Traces          agent.TraceSink // optional; receives traces of failed voice handling
	STTModel        string          // recorded as the model in voice failure traces
}

// Handler is safe for concurrent use.
type Handler struct {
	msg     Messenger
	agent   AgentRunner
	members MemberChecker
	stt     stt.Transcriber // nil disables voice
	cfg     Config
	log     *slog.Logger
}

// NewHandler applies defaults. A nil transcriber disables voice messages.
func NewHandler(msg Messenger, ag AgentRunner, members MemberChecker, transcriber stt.Transcriber, cfg Config, log *slog.Logger) *Handler {
	if cfg.MaxVoiceSeconds == 0 {
		cfg.MaxVoiceSeconds = 60
	}
	if cfg.TypingInterval == 0 {
		cfg.TypingInterval = 4 * time.Second
	}
	if cfg.MessageTimeout == 0 {
		cfg.MessageTimeout = 2 * time.Minute
	}
	if log == nil {
		log = slog.Default()
	}
	return &Handler{msg: msg, agent: ag, members: members, stt: transcriber, cfg: cfg, log: log}
}

// Handle processes one incoming message end to end.
func (h *Handler) Handle(ctx context.Context, in Incoming) {
	ctx, cancel := context.WithTimeout(ctx, h.cfg.MessageTimeout)
	defer cancel()

	// Expenses are handled in private chats only: a reply in a group would be
	// visible to everyone there. Cut off before membership, STT and the agent.
	// Fail closed: an unknown chat type is treated as non-private.
	if in.ChatType != chatTypePrivate {
		if in.ChatType == "group" || in.ChatType == "supergroup" {
			h.reply(ctx, in.ChatID, textPrivateOnly)
		}
		return
	}

	ok, err := h.members.MemberExists(ctx, in.UserID)
	if err != nil {
		h.log.ErrorContext(ctx, "member check failed", "user_id", in.UserID, "err", err)
		h.reply(ctx, in.ChatID, textInternalError)
		return
	}
	if !ok {
		h.log.InfoContext(ctx, "message from non-member", "user_id", in.UserID)
		h.reply(ctx, in.ChatID, fmt.Sprintf(textNotMember, in.UserID))
		return
	}

	text := strings.TrimSpace(in.Text)
	switch {
	case in.Voice != nil:
		h.handleVoice(ctx, in)
	case text == "/start":
		h.reply(ctx, in.ChatID, textStart)
	case text != "":
		stop := h.typing(ctx, in.ChatID)
		defer stop()
		h.runAgent(ctx, in.ChatID, agent.Input{UserID: in.UserID, Text: text, Source: agent.SourceText}, "")
	default:
		h.reply(ctx, in.ChatID, textUnsupported)
	}
}

func (h *Handler) handleVoice(ctx context.Context, in Incoming) {
	if h.stt == nil {
		h.reply(ctx, in.ChatID, textVoiceDisabled)
		return
	}
	if in.Voice.Duration > h.cfg.MaxVoiceSeconds {
		h.reply(ctx, in.ChatID, fmt.Sprintf(textVoiceTooLong, h.cfg.MaxVoiceSeconds))
		return
	}
	stop := h.typing(ctx, in.ChatID)
	defer stop()

	start := time.Now()
	audio, err := h.msg.DownloadFile(ctx, in.Voice.FileID)
	if err != nil {
		h.log.ErrorContext(ctx, "voice download failed", "user_id", in.UserID, "err", err)
		h.traceVoiceFailure(ctx, in, "download voice: "+err.Error(), nil, start)
		h.reply(ctx, in.ChatID, textVoiceFailed)
		return
	}
	sttStart := time.Now()
	text, err := h.stt.Transcribe(ctx, audio, "ogg")
	sttMs := int(time.Since(sttStart).Milliseconds())
	text = strings.TrimSpace(text)
	if err != nil || text == "" {
		h.log.WarnContext(ctx, "voice transcription failed", "user_id", in.UserID, "err", err)
		msg := "empty transcript"
		if err != nil {
			msg = "transcribe: " + err.Error()
		}
		h.traceVoiceFailure(ctx, in, msg, &sttMs, start)
		h.reply(ctx, in.ChatID, textVoiceFailed)
		return
	}
	h.runAgent(ctx, in.ChatID, agent.Input{UserID: in.UserID, Text: text, Source: agent.SourceVoice, STTms: &sttMs}, "🎤 «"+text+"»\n\n")
}

// traceVoiceFailure records a failed voice message; no-op without a trace sink.
func (h *Handler) traceVoiceFailure(ctx context.Context, in Incoming, errText string, sttMs *int, start time.Time) {
	if h.cfg.Traces == nil {
		return
	}
	t := domain.Trace{
		UserID: in.UserID, Source: string(agent.SourceVoice), Model: h.cfg.STTModel,
		STTms: sttMs, LatencyMs: int(time.Since(start).Milliseconds()), Error: errText,
	}
	if err := h.cfg.Traces.SaveTrace(context.WithoutCancel(ctx), t); err != nil {
		h.log.ErrorContext(ctx, "save voice trace", "err", err)
	}
}

func (h *Handler) runAgent(ctx context.Context, chatID int64, in agent.Input, prefix string) {
	res, err := h.agent.Run(ctx, in)
	if err != nil {
		h.log.ErrorContext(ctx, "agent failed", "user_id", in.UserID, "err", err)
		// After completed changes the agent provides a factual summary; never hide them behind a generic error.
		if !errors.Is(err, agent.ErrPartial) || strings.TrimSpace(res.Reply) == "" {
			h.reply(ctx, chatID, prefix+textInternalError)
			return
		}
	}
	h.reply(ctx, chatID, prefix+res.Reply)
}

func (h *Handler) reply(ctx context.Context, chatID int64, text string) {
	if utf8.RuneCountInString(text) > maxMessageRunes {
		text = string([]rune(text)[:maxMessageRunes-1]) + "…"
	}
	// Detached: the message ctx may already be expired (e.g. an LLM outage), and the member must still get an answer.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), replyTimeout)
	defer cancel()
	if err := h.msg.SendText(ctx, chatID, text); err != nil {
		h.log.ErrorContext(ctx, "send message failed", "chat_id", chatID, "err", err)
	}
}

// typing shows the "typing…" status until the returned stop func is called.
func (h *Handler) typing(ctx context.Context, chatID int64) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(h.cfg.TypingInterval)
		defer ticker.Stop()
		for {
			if err := h.msg.SendTyping(ctx, chatID); err != nil && ctx.Err() == nil {
				h.log.DebugContext(ctx, "send typing failed", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
