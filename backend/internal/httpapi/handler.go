package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

// Deps are the handler's collaborators.
type Deps struct {
	Service   *expense.Service
	Traces    TraceLister
	Bus       *events.Bus
	Health    func(context.Context) error
	Log       *slog.Logger
	Heartbeat time.Duration // SSE keep-alive interval; default 25s
	// StreamsCtx ends all SSE streams when cancelled (server shutdown); default never.
	StreamsCtx context.Context
}

// NewHandler builds the full HTTP handler: REST under /api, SSE at /api/events, /healthz.
func NewHandler(d Deps) http.Handler {
	if d.Heartbeat == 0 {
		d.Heartbeat = 25 * time.Second
	}
	if d.StreamsCtx == nil {
		d.StreamsCtx = context.Background()
	}
	mux := http.NewServeMux()
	strict := gen.NewStrictHandlerWithOptions(&Server{svc: d.Service, traces: d.Traces}, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequest,
		ResponseErrorHandlerFunc: errorHandler(d.Log),
	})
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{BaseURL: "/api", BaseRouter: mux, ErrorHandlerFunc: badRequest})
	mux.Handle("GET /api/events", sseHandler(d.StreamsCtx, d.Bus, d.Heartbeat, d.Log))
	mux.Handle("GET /healthz", healthHandler(d.Health))
	validated, err := validateRequests("/api", mux)
	if err != nil {
		panic(err) // the spec is embedded at build time, so this is a programming error
	}
	return recoverer(d.Log, logRequests(d.Log, validated))
}
