package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// OpenAIConfig configures an OpenAI-compatible endpoint (OpenRouter, OpenAI, Ollama, vLLM…).
type OpenAIConfig struct {
	BaseURL    string // e.g. https://openrouter.ai/api/v1
	APIKey     string
	Timeout    time.Duration // per attempt; default 30s
	MaxRetries int           // extra attempts on 429/5xx/network errors
	RetryBase  time.Duration // first backoff delay, doubled each retry; default 500ms
	HTTPClient *http.Client
	Referer    string // optional OpenRouter attribution headers
	Title      string
}

// OpenAI implements Client over HTTP.
type OpenAI struct {
	cfg  OpenAIConfig
	http *http.Client
}

// NewOpenAI returns a client with defaults applied.
func NewOpenAI(cfg OpenAIConfig) *OpenAI {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.RetryBase == 0 {
		cfg.RetryBase = 500 * time.Millisecond
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	return &OpenAI{cfg: cfg, http: hc}
}

type chatPayload struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
}

type chatResult struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
	Error *struct {
		Message string          `json:"message"`
		Code    json.RawMessage `json:"code"`
	} `json:"error"`
}

func (c *OpenAI) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	p := chatPayload{Model: req.Model, Messages: req.Messages, Tools: req.Tools, Temperature: req.Temperature}
	if len(req.Tools) > 0 {
		p.ToolChoice = "auto"
	}
	body, err := json.Marshal(p)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ChatResponse{}, ctx.Err()
			case <-time.After(c.cfg.RetryBase << (attempt - 1)):
			}
		}
		resp, err := c.once(ctx, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if ctx.Err() != nil || !retryable(err) {
			break
		}
	}
	return ChatResponse{}, lastErr
}

func (c *OpenAI) once(ctx context.Context, body []byte) (ChatResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.Referer != "" {
		req.Header.Set("HTTP-Referer", c.cfg.Referer)
	}
	if c.cfg.Title != "" {
		req.Header.Set("X-Title", c.cfg.Title)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("read llm response: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return ChatResponse{}, &APIError{Status: resp.StatusCode, Message: truncate(string(data), 500)}
	}

	var r chatResult
	if err := json.Unmarshal(data, &r); err != nil {
		return ChatResponse{}, fmt.Errorf("decode llm response: %w", err)
	}
	if r.Error != nil {
		status, convErr := strconv.Atoi(strings.Trim(string(r.Error.Code), `"`))
		if convErr != nil || status == 0 {
			status = http.StatusBadGateway
		}
		return ChatResponse{}, &APIError{Status: status, Message: r.Error.Message}
	}
	if len(r.Choices) == 0 {
		return ChatResponse{}, &APIError{Status: http.StatusBadGateway, Message: "empty choices"}
	}
	msg := r.Choices[0].Message
	msg.Role = RoleAssistant
	return ChatResponse{Message: msg, Usage: r.Usage}, nil
}

func retryable(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == http.StatusTooManyRequests || apiErr.Status >= 500
	}
	return !errors.Is(err, context.Canceled) // network errors and per-attempt timeouts
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
