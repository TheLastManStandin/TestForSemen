package evals

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

// fakeLLM adds "кофе 350" via a tool call; for any other input it asks a question.
type fakeLLM struct{}

func (fakeLLM) Chat(_ context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	last := req.Messages[len(req.Messages)-1]
	switch {
	case last.Role == llm.RoleTool:
		return llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: "Записал: кофе — 350 ₽"}, Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 5}}, nil
	case last.Content == "кофе 350":
		return llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID: "1", Type: "function", Function: llm.FunctionCall{Name: "add_expense", Arguments: `{"amount":"350","category":"food","comment":"кофе"}`},
		}}}, Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 5}}, nil
	case last.Content == "сломайся":
		return llm.ChatResponse{}, errors.New("provider down")
	default:
		return llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: "Сколько стоило?"}}, nil
	}
}

func TestRunnerPassesAndFails(t *testing.T) {
	cases := []Case{
		{Name: "add", Group: "add", Input: "кофе 350", Expect: Expect{Created: []ExpenseMatch{{Amount: "350", Category: "food", Date: "today"}}, Tools: []string{"add_expense"}}},
		{Name: "ask", Group: "add", Input: "купил кофе", Expect: Expect{Ask: true}},
		{Name: "wrong", Group: "query", Input: "сколько на еду", Expect: Expect{ReplyContains: []string{"4 400"}}},
	}
	results := (&Runner{LLM: fakeLLM{}, Model: "fake", Parallel: 2}).Run(context.Background(), cases)
	require.Len(t, results, 3)
	assert.True(t, results[0].Passed(), results[0].Failures)
	assert.Equal(t, []string{"add_expense"}, results[0].Tools)
	assert.Equal(t, 20, results[0].Usage.PromptTokens)
	assert.True(t, results[1].Passed(), results[1].Failures)
	assert.False(t, results[2].Passed())
	assert.Contains(t, results[2].Failures[0], "4 400")
}

func TestRunnerUsesHistory(t *testing.T) {
	var seen []llm.Message
	spy := llmFunc(func(_ context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
		seen = req.Messages
		return llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"}}, nil
	})
	c := Case{Name: "h", Group: "g", Input: "а 450", History: []Turn{{Role: "user", Content: "кофе 350"}, {Role: "assistant", Content: "Записал"}}}
	results := (&Runner{LLM: spy, Model: "spy", Parallel: 1}).Run(context.Background(), []Case{c})
	require.Empty(t, results[0].Err)
	require.Len(t, seen, 4)
	assert.Equal(t, "кофе 350", seen[1].Content)
	assert.Equal(t, llm.RoleAssistant, seen[2].Role)
}

func TestRunnerReportsErrorsPerCase(t *testing.T) {
	cases := []Case{
		{Name: "boom", Group: "g", Input: "сломайся"},
		{Name: "ok", Group: "g", Input: "купил кофе", Expect: Expect{Ask: true}},
	}
	results := (&Runner{LLM: fakeLLM{}, Model: "fake", Parallel: 2}).Run(context.Background(), cases)
	assert.Contains(t, results[0].Err, "provider down")
	assert.False(t, results[0].Passed())
	assert.True(t, results[1].Passed())
}

func TestRunnerSeparatesProviderErrors(t *testing.T) {
	errs := map[string]error{
		"rate":     &llm.APIError{Status: 429, Message: "rate limited"},
		"gateway":  &llm.APIError{Status: 502, Message: "bad gateway"},
		"timeout":  fmt.Errorf("llm request: %w", context.DeadlineExceeded),
		"conn":     fmt.Errorf("llm request: %w", &url.Error{Op: "Post", URL: "https://x", Err: errors.New("connection reset")}),
		"badreq":   &llm.APIError{Status: 400, Message: "invalid tool schema"},
		"internal": errors.New("boom"),
	}
	var cases []Case
	for name := range errs {
		cases = append(cases, Case{Name: name, Group: "g", Input: name})
	}
	fail := llmFunc(func(_ context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
		return llm.ChatResponse{}, errs[req.Messages[len(req.Messages)-1].Content]
	})
	for _, r := range (&Runner{LLM: fail, Model: "f", Parallel: 2}).Run(context.Background(), cases) {
		want := r.Case.Name != "badreq" && r.Case.Name != "internal"
		assert.Equal(t, want, r.ProviderError, r.Case.Name)
		assert.NotEmpty(t, r.Err, r.Case.Name)
		assert.False(t, r.Passed(), r.Case.Name)
	}
}

type llmFunc func(context.Context, llm.ChatRequest) (llm.ChatResponse, error)

func (f llmFunc) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	return f(ctx, req)
}
