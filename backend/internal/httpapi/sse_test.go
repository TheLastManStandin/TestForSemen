package httpapi_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi"
)

func readUntil(t *testing.T, r *bufio.Reader, prefix string) string {
	t.Helper()
	for {
		line, err := r.ReadString('\n')
		require.NoError(t, err, "stream ended before %q", prefix)
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
}

func TestEventsStream(t *testing.T) {
	e := newEnv(t, func(d *httpapi.Deps) { d.Heartbeat = 50 * time.Millisecond })
	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, e.srv.URL+"/api/events", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	r := bufio.NewReader(resp.Body)
	readUntil(t, r, ": connected")

	_, err = e.svc.Create(reqCtx, expense.CreateInput{UserID: papa, Amount: 35000, Category: domain.CategoryFood, Comment: "кофе"})
	require.NoError(t, err)

	readUntil(t, r, "event: expense.created")
	data := readUntil(t, r, "data: ")
	assert.Contains(t, data, `"type":"expense.created"`)
	assert.Contains(t, data, `"comment":"кофе"`)
	assert.Contains(t, data, `"amount":"350.00"`)

	readUntil(t, r, ": ping")
}

func TestEventsStreamEndsOnShutdownContext(t *testing.T) {
	streams, endStreams := context.WithCancel(context.Background())
	defer endStreams()
	e := newEnv(t, func(d *httpapi.Deps) { d.StreamsCtx = streams })
	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, e.srv.URL+"/api/events", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	r := bufio.NewReader(resp.Body)
	readUntil(t, r, ": connected")

	endStreams()
	_, err = io.ReadAll(r) // returns once the server closes the stream
	require.NoError(t, err)
	require.NoError(t, reqCtx.Err(), "stream must end before the client gives up")
}

func TestHealthz(t *testing.T) {
	e := newEnv(t)
	resp, data := e.do(t, http.MethodGet, "/healthz", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"status":"ok"}`, string(data))

	down := newEnv(t, func(d *httpapi.Deps) {
		d.Health = func(context.Context) error { return errors.New("db down") }
	})
	resp, data = down.do(t, http.MethodGet, "/healthz", nil)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.JSONEq(t, `{"status":"unavailable"}`, string(data))
}
