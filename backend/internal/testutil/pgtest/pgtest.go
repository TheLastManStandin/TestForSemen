// Package pgtest provides a shared, migrated Postgres testcontainer.
package pgtest

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres"
)

var (
	once     sync.Once
	pool     *pgxpool.Pool
	startErr error
)

// Pool returns a migrated pool with all tables truncated. The container is shared
// within the test binary and removed by testcontainers' reaper afterwards.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: requires Docker")
	}
	once.Do(start)
	if startErr != nil {
		t.Fatalf("start postgres: %v", startErr)
	}
	_, err := pool.Exec(context.Background(),
		"TRUNCATE agent_traces, chat_messages, expenses, members RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

func start() {
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("expenses"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		startErr = err
		return
	}
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		startErr = err
		return
	}
	p, err := postgres.Open(ctx, dsn)
	if err != nil {
		startErr = err
		return
	}
	if err := postgres.Migrate(ctx, p); err != nil {
		startErr = err
		return
	}
	pool = p
}
