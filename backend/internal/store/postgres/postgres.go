// Package postgres implements persistence on PostgreSQL using sqlc-generated queries.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres/db"
	"github.com/TheLastManStandin/TestForSemen/backend/migrations"
)

// Store implements expense.Repository and the agent's chat/trace storage.
type Store struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// New wraps a pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: db.New(pool)}
}

// Open connects and waits up to ~10s for the database to answer.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	var pingErr error
	for range 10 {
		if pingErr = pool.Ping(ctx); pingErr == nil {
			return pool, nil
		}
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	pool.Close()
	return nil, fmt.Errorf("ping postgres: %w", pingErr)
}

// Migrate applies all pending goose migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
