package events_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
)

func receive(t *testing.T, ch <-chan events.Event) events.Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(time.Second):
		t.Fatal("no event received")
		return events.Event{}
	}
}

func TestPublishFansOut(t *testing.T) {
	bus := events.NewBus()
	a, unsubA := bus.Subscribe()
	defer unsubA()
	b, unsubB := bus.Subscribe()
	defer unsubB()

	bus.Publish(events.Event{Type: events.ExpenseCreated, Expense: domain.Expense{ID: 7}})

	assert.Equal(t, int64(7), receive(t, a).Expense.ID)
	assert.Equal(t, events.ExpenseCreated, receive(t, b).Type)
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	bus := events.NewBus()
	ch, unsub := bus.Subscribe()
	unsub()
	unsub() // idempotent
	_, ok := <-ch
	assert.False(t, ok)
	bus.Publish(events.Event{Type: events.ExpenseDeleted}) // must not panic
}

func TestSlowSubscriberDoesNotBlockPublisher(t *testing.T) {
	bus := events.NewBus()
	_, unsub := bus.Subscribe() // never read
	defer unsub()

	done := make(chan struct{})
	go func() {
		for range 1000 {
			bus.Publish(events.Event{Type: events.ExpenseUpdated})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "publisher blocked by slow subscriber")
	}
}
