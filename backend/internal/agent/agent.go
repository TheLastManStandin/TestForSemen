// Package agent turns a member's free-form message into expense operations via LLM tool calling.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"strconv"
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
// a trace is saved in every case. Changes are confirmed in Result.Reply from the tool
// results, not from the model's wording. If the failure came after the turn had already
// changed data, the error wraps ErrPartial and Result.Reply is a factual summary of the
// changes for the member. Hitting the step limit or an empty model reply is not an error:
// Reply is then a polite fallback, or the summary of changes if there were any.
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
	if errors.Is(err, ErrTooManySteps) || errors.Is(err, ErrEmptyReply) {
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
	var failures []string                          // why calls of the latest tool batch failed, for the reply
	runID := strconv.FormatUint(rand.Uint64(), 36) // makes generated tool call ids unique in history
	temperature := a.cfg.Temperature
	for iter := range a.cfg.MaxIterations {
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
		reply.ToolCalls = uniqueCalls(reply.ToolCalls, fmt.Sprintf("%s_%d", runID, iter))
		msgs = append(msgs, reply)
		turn = append(turn, reply)

		if len(reply.ToolCalls) == 0 {
			res.Reply, err = composeReply(reply.Content, res.Changes, failures, a.svc.Today())
			// History keeps what the member saw: it is never empty (some providers reject an empty
			// assistant text), and confirmations the model might copy from it are dropped anyway.
			turn[len(turn)-1].Content = res.Reply
			a.saveTurn(ctx, in.UserID, turn)
			return res, err
		}
		failures = failures[:0]
		for _, call := range reply.ToolCalls {
			o := a.execute(ctx, in.UserID, call)
			res.Steps = append(res.Steps, o.step)
			if o.change != nil {
				res.Changes = append(res.Changes, *o.change)
			}
			if o.failure != "" && !slices.Contains(failures, o.failure) {
				failures = append(failures, o.failure)
			}
			toolMsg := llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: o.content}
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

// uniqueCalls makes tool call ids unique within one response, so every call keeps exactly one
// result. A call repeated with the same id, name and arguments is a duplicate and runs once;
// a different call that reuses an id (some providers give all parallel calls one id) gets a new
// id; missing ids are filled in. Ids are compared only within a response: some providers
// number calls anew in each one. prefix is unique per run and step, so generated ids do not
// repeat across turns kept in history.
func uniqueCalls(calls []llm.ToolCall, prefix string) []llm.ToolCall {
	provided := make(map[string]bool, len(calls)) // generated ids must not take one of these
	for _, c := range calls {
		provided[c.ID] = true
	}
	seen := make(map[string]llm.FunctionCall, len(calls))
	taken := func(id string) bool {
		_, ok := seen[id]
		return ok || provided[id]
	}
	out := make([]llm.ToolCall, 0, len(calls))
	for i, c := range calls {
		fn, dup := seen[c.ID]
		if dup && fn == c.Function {
			continue
		}
		if dup || c.ID == "" {
			c.ID = fmt.Sprintf("call_%s_%d", prefix, i)
			for n := 0; taken(c.ID); n++ {
				c.ID = fmt.Sprintf("call_%s_%d_%d", prefix, i, n)
			}
		}
		seen[c.ID] = c.Function
		out = append(out, c)
	}
	return out
}
