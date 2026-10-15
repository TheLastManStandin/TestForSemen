package agent

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// MemoryStore is an in-memory History and TraceSink for tests and evals.
type MemoryStore struct {
	mu     sync.Mutex
	now    func() time.Time
	chats  map[int64][]storedMessage
	traces []domain.Trace
}

type storedMessage struct {
	at      time.Time
	content json.RawMessage
}

// NewMemoryStore uses now to timestamp messages (pass the same clock as the service).
func NewMemoryStore(now func() time.Time) *MemoryStore {
	if now == nil {
		now = time.Now
	}
	return &MemoryStore{now: now, chats: map[int64][]storedMessage{}}
}

func (s *MemoryStore) RecentChat(_ context.Context, userID int64, since time.Time, limit int) ([]json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []json.RawMessage
	for _, m := range s.chats[userID] {
		if !m.at.Before(since) {
			out = append(out, m.content)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

func (s *MemoryStore) AppendChat(_ context.Context, userID int64, msgs []json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	at := s.now()
	for _, m := range msgs {
		s.chats[userID] = append(s.chats[userID], storedMessage{at: at, content: m})
	}
	return nil
}

func (s *MemoryStore) SaveTrace(_ context.Context, t domain.Trace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.ID = int64(len(s.traces) + 1)
	t.CreatedAt = s.now()
	s.traces = append(s.traces, t)
	return nil
}

// Traces returns a copy of saved traces in insertion order.
func (s *MemoryStore) Traces() []domain.Trace {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.Trace(nil), s.traces...)
}
