// Package llm is a minimal provider-agnostic chat-completions client with tool calling.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

// Role of a chat message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message uses the chat-completions wire format, so it can be stored and replayed as-is.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON object encoded as a string
}

type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

type ChatRequest struct {
	Model       string
	Messages    []Message
	Tools       []Tool
	Temperature *float64
}

type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Cost             float64 `json:"cost,omitempty"` // USD, when the provider reports it (OpenRouter)
}

// Add accumulates o into u.
func (u *Usage) Add(o Usage) {
	u.PromptTokens += o.PromptTokens
	u.CompletionTokens += o.CompletionTokens
	u.Cost += o.Cost
}

type ChatResponse struct {
	Message Message
	Usage   Usage
}

// Client sends one chat-completions request.
type Client interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// APIError is an error reported by the provider (HTTP status or in-body error).
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("llm api error %d: %s", e.Status, e.Message) }
