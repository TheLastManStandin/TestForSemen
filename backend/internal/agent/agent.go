// Package agent turns a member's free-form message into expense operations via LLM tool calling.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

// History stores the conversation in chat-completions JSON.
type History interface {
	RecentChat(ctx context.Context, userID int64, since time.Time, limit int) ([]json.RawMessage, error)
	AppendChat(ctx context.Context, userID int64, msgs []json.RawMessage) error
}

// TraceSink stores one record per agent run.
type TraceSink interface {
	SaveTrace(ctx context.Context, t domain.Trace) error
}

// Config tunes the agent loop.
type Config struct {
	Model         string
	MaxIterations int           // LLM calls per message; default 6
	HistoryLimit  int           // messages; default 20
	HistoryWindow time.Duration // default 6h
	Temperature   float64
}

// Source tells whether the input was typed or transcribed.
type Source string

const (
	SourceText  Source = "text"
	SourceVoice Source = "voice"
)

// Input is one member message.
type Input struct {
	UserID int64 // from the Telegram update, never from the model
	Text   string
	Source Source
	STTms  *int // transcription latency for voice input
}

// Result is the reply for the member plus run details.
type Result struct {
	Reply   string
	Steps   []domain.TraceStep
	Usage   llm.Usage
	Changes []Change // successful add/update/delete calls, in order; may be non-empty with a nil error (step limit)
}

// ErrTooManySteps is recorded in the trace when the loop hits MaxIterations.
var ErrTooManySteps = errors.New("agent: too many tool iterations")

// ReplyTooManySteps is sent to the member when the loop gives up.
const ReplyTooManySteps = "Не получилось разобраться с запросом, попробуй сформулировать иначе"

const replyEmpty = "Готово."

// Agent is safe for concurrent use; runs for the same member are serialized.
type Agent struct {
	llm     llm.Client
	svc     *expense.Service
	history History
	traces  TraceSink
	cfg     Config
	log     *slog.Logger
	locks   sync.Map // userID -> *sync.Mutex
}

// New builds an agent with defaults applied.
func New(client llm.Client, svc *expense.Service, history History, traces TraceSink, cfg Config, log *slog.Logger) *Agent {
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 6
	}
	if cfg.HistoryLimit == 0 {
		cfg.HistoryLimit = 20
	}
	if cfg.HistoryWindow == 0 {
		cfg.HistoryWindow = 6 * time.Hour
	}
	if log == nil {
		log = slog.Default()
	}
	return &Agent{llm: client, svc: svc, history: history, traces: traces, cfg: cfg, log: log}
}

// Run handles one message. The error is non-nil only for infrastructure failures;
// a trace is saved in every case. If the failure came after the turn had already changed
// data, the error wraps ErrPartial and Result.Reply is a factual summary of the changes
// for the member. Hitting the step limit is not an error: Reply is then ReplyTooManySteps,
// or the summary of changes if there were any.
func (a *Agent) Run(ctx context.Context, in Input) (Result, error) {
	if in.Source == "" {
		in.Source = SourceText
	}
	mu := a.userLock(in.UserID)
	mu.Lock()
	defer mu.Unlock()

	start := time.Now()
	res, err := a.run(ctx, in)

	trace := domain.Trace{
		UserID: in.UserID, Source: string(in.Source), Input: in.Text, Output: res.Reply, Steps: res.Steps,
		Model: a.cfg.Model, PromptTokens: res.Usage.PromptTokens, CompletionTokens: res.Usage.CompletionTokens,
		STTms: in.STTms, LatencyMs: int(time.Since(start).Milliseconds()),
	}
	if err != nil {
		trace.Error = err.Error()
	}
	if saveErr := a.traces.SaveTrace(context.WithoutCancel(ctx), trace); saveErr != nil {
		a.log.ErrorContext(ctx, "save trace", "err", saveErr)
	}
	if errors.Is(err, ErrTooManySteps) {
		return res, nil
	}
	return res, err
}

func (a *Agent) run(ctx context.Context, in Input) (Result, error) {
	now := a.svc.Now()
	history, err := a.loadHistory(ctx, in.UserID, now)
	if err != nil {
		return Result{}, err
	}

	user := llm.Message{Role: llm.RoleUser, Content: in.Text}
	msgs := make([]llm.Message, 0, len(history)+8)
	msgs = append(msgs, llm.Message{Role: llm.RoleSystem, Content: BuildSystemPrompt(now)})
	msgs = append(msgs, history...)
	msgs = append(msgs, user)
	turn := []llm.Message{user}

	var res Result
	temperature := a.cfg.Temperature
	for range a.cfg.MaxIterations {
		resp, err := a.llm.Chat(ctx, llm.ChatRequest{Model: a.cfg.Model, Messages: msgs, Tools: toolDefs, Temperature: &temperature})
		if err != nil {
			err = fmt.Errorf("llm: %w", err)
			if len(res.Changes) == 0 {
				return res, err // nothing changed: the turn is dropped and retrying is safe
			}
			// Close the turn with the summary so the next turn knows what was done.
			res.Reply = summarizeChanges(res.Changes, a.svc.Today())
			turn = append(turn, llm.Message{Role: llm.RoleAssistant, Content: res.Reply})
			a.saveTurn(ctx, in.UserID, turn)
			return res, fmt.Errorf("%w: %w", ErrPartial, err)
		}
		res.Usage.Add(resp.Usage)

		reply := resp.Message
		reply.Role = llm.RoleAssistant
		msgs = append(msgs, reply)
		turn = append(turn, reply)

		if len(reply.ToolCalls) == 0 {
			res.Reply = strings.TrimSpace(reply.Content)
			if res.Reply == "" {
				res.Reply = replyEmpty
			}
			a.saveTurn(ctx, in.UserID, turn)
			return res, nil
		}
		for _, call := range reply.ToolCalls {
			out, step, change := a.execute(ctx, in.UserID, call)
			res.Steps = append(res.Steps, step)
			if change != nil {
				res.Changes = append(res.Changes, *change)
			}
			toolMsg := llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: out}
			msgs = append(msgs, toolMsg)
			turn = append(turn, toolMsg)
		}
	}

	res.Reply = ReplyTooManySteps
	err = ErrTooManySteps
	if len(res.Changes) > 0 {
		res.Reply = summarizeChanges(res.Changes, a.svc.Today())
		err = fmt.Errorf("%w: %w", ErrPartial, err)
	}
	turn = append(turn, llm.Message{Role: llm.RoleAssistant, Content: res.Reply})
	a.saveTurn(ctx, in.UserID, turn)
	return res, err
}

// loadHistory returns recent messages starting at a user message, so a tool result
// is never sent without the assistant call that produced it.
func (a *Agent) loadHistory(ctx context.Context, userID int64, now time.Time) ([]llm.Message, error) {
	raw, err := a.history.RecentChat(ctx, userID, now.Add(-a.cfg.HistoryWindow), a.cfg.HistoryLimit)
	if err != nil {
		return nil, fmt.Errorf("load history: %w", err)
	}
	msgs := make([]llm.Message, 0, len(raw))
	for _, r := range raw {
		var m llm.Message
		if err := json.Unmarshal(r, &m); err == nil {
			msgs = append(msgs, m)
		}
	}
	for i, m := range msgs {
		if m.Role == llm.RoleUser {
			return msgs[i:], nil
		}
	}
	return nil, nil
}

func (a *Agent) saveTurn(ctx context.Context, userID int64, turn []llm.Message) {
	raw := make([]json.RawMessage, 0, len(turn))
	for _, m := range turn {
		if b, err := json.Marshal(m); err == nil {
			raw = append(raw, b)
		}
	}
	if err := a.history.AppendChat(context.WithoutCancel(ctx), userID, raw); err != nil {
		a.log.ErrorContext(ctx, "save history", "err", err)
	}
}

func (a *Agent) userLock(userID int64) *sync.Mutex {
	v, _ := a.locks.LoadOrStore(userID, &sync.Mutex{})
	return v.(*sync.Mutex)
}
