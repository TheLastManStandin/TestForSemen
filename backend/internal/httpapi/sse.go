package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

type sseEvent struct {
	Type    string      `json:"type"`
	Expense gen.Expense `json:"expense"`
}

// sseHandler streams expense changes until the client leaves or streams is cancelled. The http.Server must not set WriteTimeout.
func sseHandler(streams context.Context, bus *events.Bus, heartbeat time.Duration, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("X-Accel-Buffering", "no") // disable nginx buffering

		ch, unsubscribe := bus.Subscribe()
		defer unsubscribe()

		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, ": connected\n\n"); err != nil {
			return
		}
		if err := rc.Flush(); err != nil {
			log.WarnContext(r.Context(), "sse: flush unsupported", "err", err)
			return
		}

		ticker := time.NewTicker(heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-streams.Done():
				return
			case <-ticker.C:
				if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
					return
				}
			case ev, ok := <-ch:
				if !ok {
					return
				}
				data, err := json.Marshal(sseEvent{Type: string(ev.Type), Expense: toAPIExpense(ev.Expense)})
				if err != nil {
					log.ErrorContext(r.Context(), "sse: marshal", "err", err)
					continue
				}
				if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data); err != nil {
					return
				}
			}
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}
