package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const maxFileBytes = 20 << 20 // Telegram bot API download limit

// Telegram adapts go-telegram/bot to Messenger and feeds updates to a Handler.
type Telegram struct {
	b       *tgbot.Bot
	token   string
	http    *http.Client
	handler *Handler
	log     *slog.Logger
}

// NewTelegram creates the client; it calls getMe, so an invalid token fails fast.
// Extra options are applied after the defaults (used by tests to point at a fake server).
func NewTelegram(token string, log *slog.Logger, opts ...tgbot.Option) (*Telegram, error) {
	t := &Telegram{token: token, http: &http.Client{Timeout: 60 * time.Second}, log: log}
	opts = append([]tgbot.Option{
		tgbot.WithDefaultHandler(t.onUpdate),
		tgbot.WithErrorsHandler(func(err error) { log.Error("telegram", "err", err) }),
	}, opts...)
	b, err := tgbot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("telegram: %w", err)
	}
	t.b = b
	return t, nil
}

// SetHandler must be called before Start.
func (t *Telegram) SetHandler(h *Handler) { t.handler = h }

// Start long-polls until ctx is cancelled. Updates are handled concurrently.
func (t *Telegram) Start(ctx context.Context) { t.b.Start(ctx) }

func (t *Telegram) onUpdate(ctx context.Context, _ *tgbot.Bot, u *models.Update) {
	in, ok := toIncoming(u)
	if !ok {
		t.log.Debug("update dropped", "update_id", u.ID)
		return
	}
	t.log.Debug("update", "update_id", u.ID, "user_id", in.UserID, "chat_id", in.ChatID)
	if t.handler == nil {
		return
	}
	t.handler.Handle(ctx, in)
}

func toIncoming(u *models.Update) (Incoming, bool) {
	if u == nil || u.Message == nil || u.Message.From == nil {
		return Incoming{}, false
	}
	m := u.Message
	in := Incoming{ChatID: m.Chat.ID, ChatType: string(m.Chat.Type), UserID: m.From.ID, Text: m.Text}
	if m.Voice != nil {
		in.Voice = &Voice{FileID: m.Voice.FileID, Duration: m.Voice.Duration}
	}
	return in, true
}

func (t *Telegram) SendText(ctx context.Context, chatID int64, text string) error {
	_, err := t.b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: text})
	return err
}

func (t *Telegram) SendTyping(ctx context.Context, chatID int64) error {
	_, err := t.b.SendChatAction(ctx, &tgbot.SendChatActionParams{ChatID: chatID, Action: models.ChatActionTyping})
	return err
}

func (t *Telegram) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	f, err := t.b.GetFile(ctx, &tgbot.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.b.FileDownloadLink(f), nil)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", t.redact(err))
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", t.redact(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download file: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if len(data) > maxFileBytes {
		return nil, fmt.Errorf("file larger than %d bytes", maxFileBytes)
	}
	return data, nil
}

// redact hides the bot token that the file download URL embeds in *url.Error.
func (t *Telegram) redact(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		ue.URL = strings.ReplaceAll(ue.URL, t.token, "***")
	}
	return err
}

var _ Messenger = (*Telegram)(nil)
