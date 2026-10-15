// Package agent turns a member's free-form message into expense operations via LLM tool calling.
package agent

import (
	"context"
	"encoding/json"
	"log/slog"
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
