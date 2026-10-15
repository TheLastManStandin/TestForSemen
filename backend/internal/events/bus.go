// Package events is an in-process pub/sub for expense changes (feeds SSE).
package events

import (
	"sync"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// Type is the SSE event name.
type Type string

const (
	ExpenseCreated Type = "expense.created"
	ExpenseUpdated Type = "expense.updated"
	ExpenseDeleted Type = "expense.deleted"
)

// Event describes a change to an expense. For deletions Expense is the deleted record.
type Event struct {
	Type    Type
	Expense domain.Expense
}

const subscriberBuffer = 32

// Bus fans events out to subscribers. A subscriber whose buffer is full misses
// events instead of blocking writers; SSE clients refetch on reconnect anyway.
type Bus struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

// NewBus returns an empty bus.
func NewBus() *Bus { return &Bus{subs: make(map[chan Event]struct{})} }

// Publish delivers e to every subscriber without blocking.
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Subscribe returns a channel of events and an idempotent unsubscribe func that closes it.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, subscriberBuffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, ch)
			close(ch)
			b.mu.Unlock()
		})
	}
}
