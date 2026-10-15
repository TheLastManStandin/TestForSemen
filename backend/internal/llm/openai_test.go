package llm_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

func newClient(url string, retries int) *llm.OpenAI {
	return llm.NewOpenAI(llm.OpenAIConfig{BaseURL: url, APIKey: "secret", Timeout: 2 * time.Second, MaxRetries: retries, RetryBase: time.Millisecond, Title: "test"})
}

var weatherTool = llm.Tool{Type: "function", Function: llm.FunctionDef{Name: "add_expense", Description: "d", Parameters: json.RawMessage(`{"type":"object"}`)}}

func TestChatSendsRequestAndParsesToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		assert.Equal(t, "test", r.Header.Get("X-Title"))
		body, _ := io.ReadAll(r.Body)
		var got map[string]any
		require.NoError(t, json.Unmarshal(body, &got))
		assert.Equal(t, "m1", got["model"])
		assert.Equal(t, "auto", got["tool_choice"])
		assert.Len(t, got["tools"], 1)
		assert.InDelta(t, 0.1, got["temperature"], 1e-9)
		msgs := got["messages"].([]any)
		assert.Equal(t, "user", msgs[0].(map[string]any)["role"])

		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":null,
			"tool_calls":[{"id":"c1","type":"function","function":{"name":"add_expense","arguments":"{\"amount\":\"350\"}"}}]}}],
			"usage":{"prompt_tokens":120,"completion_tokens":15,"cost":0.0001}}`))
	}))
	defer srv.Close()

	temp := 0.1
	resp, err := newClient(srv.URL, 0).Chat(t.Context(), llm.ChatRequest{
		Model: "m1", Messages: []llm.Message{{Role: llm.RoleUser, Content: "кофе 350"}}, Tools: []llm.Tool{weatherTool}, Temperature: &temp,
	})
	require.NoError(t, err)
	assert.Equal(t, llm.RoleAssistant, resp.Message.Role)
	assert.Empty(t, resp.Message.Content)
	require.Len(t, resp.Message.ToolCalls, 1)
	assert.Equal(t, "c1", resp.Message.ToolCalls[0].ID)
	assert.Equal(t, "add_expense", resp.Message.ToolCalls[0].Function.Name)
	assert.JSONEq(t, `{"amount":"350"}`, resp.Message.ToolCalls[0].Function.Arguments)
	assert.Equal(t, 120, resp.Usage.PromptTokens)
	assert.Equal(t, 15, resp.Usage.CompletionTokens)
}

func TestChatWithoutToolsOmitsToolChoice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.NotContains(t, string(body), "tool_choice")
		assert.NotContains(t, string(body), `"tools"`)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Привет"}}],"usage":{}}`))
	}))
	defer srv.Close()

	resp, err := newClient(srv.URL, 0).Chat(t.Context(), llm.ChatRequest{Model: "m", Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}}})
	require.NoError(t, err)
	assert.Equal(t, "Привет", resp.Message.Content)
}

func TestChatRetriesOn5xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			http.Error(w, "upstream", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	resp, err := newClient(srv.URL, 2).Chat(t.Context(), llm.ChatRequest{Model: "m"})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Message.Content)
	assert.Equal(t, int32(3), calls.Load())
}

func TestChatDoesNotRetry4xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, `{"error":{"message":"bad model"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := newClient(srv.URL, 2).Chat(t.Context(), llm.ChatRequest{Model: "m"})
	var apiErr *llm.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
	assert.Contains(t, apiErr.Message, "bad model")
	assert.Equal(t, int32(1), calls.Load())
}

func TestChatErrorInBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited","code":429}}`))
	}))
	defer srv.Close()

	_, err := newClient(srv.URL, 0).Chat(t.Context(), llm.ChatRequest{Model: "m"})
	var apiErr *llm.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 429, apiErr.Status)
}

func TestUsageAdd(t *testing.T) {
	u := llm.Usage{PromptTokens: 1, CompletionTokens: 2, Cost: 0.5}
	u.Add(llm.Usage{PromptTokens: 10, CompletionTokens: 20, Cost: 0.25})
	assert.Equal(t, llm.Usage{PromptTokens: 11, CompletionTokens: 22, Cost: 0.75}, u)
}

func TestChatRetriesOn429(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	resp, err := newClient(srv.URL, 2).Chat(t.Context(), llm.ChatRequest{Model: "m"})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Message.Content)
	assert.Equal(t, int32(2), calls.Load())
}

func TestChatRetryExhaustionReturnsLastAPIError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := newClient(srv.URL, 2).Chat(t.Context(), llm.ChatRequest{Model: "m"})
	var apiErr *llm.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusServiceUnavailable, apiErr.Status)
	assert.Equal(t, int32(3), calls.Load())
}

func TestChatDoesNotRetryAttemptTimeout(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()

	c := llm.NewOpenAI(llm.OpenAIConfig{BaseURL: srv.URL, APIKey: "secret", Timeout: 50 * time.Millisecond, MaxRetries: 2, RetryBase: time.Millisecond})
	_, err := c.Chat(t.Context(), llm.ChatRequest{Model: "m"})
	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load())
}

func TestChatErrorsDoNotLeakAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	url := srv.URL
	_, errHTTP := newClient(url, 1).Chat(t.Context(), llm.ChatRequest{Model: "m"})
	srv.Close()
	_, errNet := newClient(url, 1).Chat(t.Context(), llm.ChatRequest{Model: "m"}) // connection refused

	for _, err := range []error{errHTTP, errNet} {
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "secret")
	}
}
