// Package stt transcribes voice messages.
package stt

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Transcriber turns audio into text.
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, format string) (string, error)
}

// OpenRouterConfig configures OpenRouter's /audio/transcriptions endpoint.
type OpenRouterConfig struct {
	BaseURL    string // e.g. https://openrouter.ai/api/v1
	APIKey     string
	Model      string // e.g. openai/whisper-large-v3
	Language   string // ISO-639-1, e.g. "ru"; empty = auto-detect
	Timeout    time.Duration
	HTTPClient *http.Client
}

// OpenRouter implements Transcriber.
type OpenRouter struct {
	cfg  OpenRouterConfig
	http *http.Client
}

// NewOpenRouter returns a transcriber with a 30s default timeout.
func NewOpenRouter(cfg OpenRouterConfig) *OpenRouter {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	return &OpenRouter{cfg: cfg, http: hc}
}

type transcriptionRequest struct {
	Model      string     `json:"model"`
	Language   string     `json:"language,omitempty"`
	InputAudio inputAudio `json:"input_audio"`
}

type inputAudio struct {
	Data   string `json:"data"`   // base64, no data: prefix
	Format string `json:"format"` // ogg, wav, mp3…
}

func (c *OpenRouter) Transcribe(ctx context.Context, audio []byte, format string) (string, error) {
	body, err := json.Marshal(transcriptionRequest{
		Model: c.cfg.Model, Language: c.cfg.Language,
		InputAudio: inputAudio{Data: base64.StdEncoding.EncodeToString(audio), Format: format},
	})
	if err != nil {
		return "", fmt.Errorf("stt: marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.BaseURL, "/")+"/audio/transcriptions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("stt: request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("stt: read: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("stt: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("stt: decode: %w", err)
	}
	return strings.TrimSpace(out.Text), nil
}
