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
}

// NewHandler builds the full HTTP handler.
func NewHandler(d Deps) http.Handler {
	if d.Heartbeat == 0 {
		d.Heartbeat = 25 * time.Second
	}
	mux := http.NewServeMux()
	strict := gen.NewStrictHandlerWithOptions(&Server{svc: d.Service, traces: d.Traces}, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequest,
		ResponseErrorHandlerFunc: errorHandler(d.Log),
	})
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{BaseURL: "/api", BaseRouter: mux, ErrorHandlerFunc: badRequest})
	return recoverer(d.Log, logRequests(d.Log, mux))
}
