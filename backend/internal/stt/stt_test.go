package stt_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/stt"
)

func TestTranscribe(t *testing.T) {
	audio := []byte("OggS fake opus bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/audio/transcriptions", r.URL.Path)
		assert.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		var body struct {
			Model      string `json:"model"`
			Language   string `json:"language"`
			InputAudio struct {
				Data   string `json:"data"`
				Format string `json:"format"`
			} `json:"input_audio"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "openai/whisper-large-v3", body.Model)
		assert.Equal(t, "ru", body.Language)
		assert.Equal(t, "ogg", body.InputAudio.Format)
		decoded, err := base64.StdEncoding.DecodeString(body.InputAudio.Data)
		require.NoError(t, err)
		assert.Equal(t, audio, decoded)
		_, _ = w.Write([]byte(`{"text":"  кофе триста пятьдесят ","usage":{"seconds":2}}`))
	}))
	defer srv.Close()

	tr := stt.NewOpenRouter(stt.OpenRouterConfig{BaseURL: srv.URL + "/", APIKey: "key", Model: "openai/whisper-large-v3", Language: "ru", Timeout: time.Second})
	text, err := tr.Transcribe(t.Context(), audio, "ogg")
	require.NoError(t, err)
	assert.Equal(t, "кофе триста пятьдесят", text)
}

func TestTranscribeHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"unsupported format"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	tr := stt.NewOpenRouter(stt.OpenRouterConfig{BaseURL: srv.URL, APIKey: "key", Model: "m"})
	_, err := tr.Transcribe(t.Context(), []byte("x"), "ogg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "unsupported format")
}
