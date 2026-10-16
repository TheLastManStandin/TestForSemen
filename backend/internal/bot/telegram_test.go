package bot

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "123456:SECRET-TOKEN"

// fakeTelegram serves getMe and getFile; file downloads are never reached
// because the test replaces the download transport.
func fakeTelegram(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_, _ = io.WriteString(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"b"}}`)
		case strings.HasSuffix(r.URL.Path, "/getFile"):
			_, _ = io.WriteString(w, `{"ok":true,"result":{"file_id":"f","file_path":"voice/a.oga"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func TestDownloadFileRedactsToken(t *testing.T) {
	srv := fakeTelegram(t)
	tg, err := NewTelegram(testToken, slog.New(slog.NewTextHandler(io.Discard, nil)), tgbot.WithServerURL(srv.URL))
	require.NoError(t, err)
	tg.http = &http.Client{Transport: failingTransport{}}

	_, err = tg.DownloadFile(context.Background(), "f")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testToken)
	assert.Contains(t, err.Error(), "***")
}

func TestDownloadFileUsesServerURL(t *testing.T) {
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_, _ = io.WriteString(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"b"}}`)
		case strings.HasSuffix(r.URL.Path, "/getFile"):
			_, _ = io.WriteString(w, `{"ok":true,"result":{"file_id":"f","file_path":"voice/a.oga"}}`)
		case strings.HasSuffix(r.URL.Path, "/voice/a.oga"):
			_, _ = io.WriteString(w, "audio")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	tg, err := NewTelegram(testToken, slog.New(slog.NewTextHandler(io.Discard, nil)), tgbot.WithServerURL(srv.URL))
	require.NoError(t, err)
	assert.Contains(t, hits[0], "/getMe")

	data, err := tg.DownloadFile(context.Background(), "f")
	require.NoError(t, err)
	assert.Equal(t, "audio", string(data))
	assert.Equal(t, "/file/bot"+testToken+"/voice/a.oga", hits[len(hits)-1])
}

func TestOnUpdateLogsIDs(t *testing.T) {
	var buf bytes.Buffer
	tg := &Telegram{log: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))}

	tg.onUpdate(context.Background(), nil, &models.Update{ID: 77, Message: &models.Message{Chat: models.Chat{ID: 10}, From: &models.User{ID: 111}, Text: "x"}})
	assert.Contains(t, buf.String(), "update_id=77")
	assert.Contains(t, buf.String(), "user_id=111")

	buf.Reset()
	tg.onUpdate(context.Background(), nil, &models.Update{ID: 78})
	assert.Contains(t, buf.String(), "update dropped")
	assert.Contains(t, buf.String(), "update_id=78")
}

func TestToIncoming(t *testing.T) {
	text := &models.Update{Message: &models.Message{Chat: models.Chat{ID: 10, Type: models.ChatTypePrivate}, From: &models.User{ID: 111}, Text: "кофе 350"}}
	in, ok := toIncoming(text)
	assert.True(t, ok)
	assert.Equal(t, Incoming{ChatID: 10, ChatType: "private", UserID: 111, Text: "кофе 350"}, in)

	voice := &models.Update{Message: &models.Message{Chat: models.Chat{ID: 10}, From: &models.User{ID: 111}, Voice: &models.Voice{FileID: "f1", Duration: 4}}}
	in, ok = toIncoming(voice)
	assert.True(t, ok)
	assert.Equal(t, &Voice{FileID: "f1", Duration: 4}, in.Voice)

	_, ok = toIncoming(&models.Update{})
	assert.False(t, ok, "updates without a message are ignored")
	_, ok = toIncoming(&models.Update{Message: &models.Message{Chat: models.Chat{ID: 10}}})
	assert.False(t, ok, "messages without a sender are ignored")

	caption := &models.Update{Message: &models.Message{Chat: models.Chat{ID: 10}, From: &models.User{ID: 111}, Caption: "look", Photo: []models.PhotoSize{{FileID: "p"}}}}
	in, ok = toIncoming(caption)
	assert.True(t, ok)
	assert.Empty(t, in.Text)
	assert.Nil(t, in.Voice)
}

func TestToIncomingCarriesChatType(t *testing.T) {
	for _, ct := range []models.ChatType{models.ChatTypePrivate, models.ChatTypeGroup, models.ChatTypeSupergroup, models.ChatTypeChannel} {
		u := &models.Update{Message: &models.Message{Chat: models.Chat{ID: 10, Type: ct}, From: &models.User{ID: 111}, Text: "x"}}
		in, ok := toIncoming(u)
		assert.True(t, ok)
		assert.Equal(t, string(ct), in.ChatType)
	}
}
