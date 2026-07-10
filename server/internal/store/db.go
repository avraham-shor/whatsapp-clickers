// Package store owns all database access. It is the only package that
// imports pgx; handlers depend on it through small interfaces.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// Store wraps the connection pool and exposes query methods.
type Store struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

// NewPool opens a pgx connection pool for the given DATABASE_URL.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	return pool, nil
}

// New builds a Store on top of an existing pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: gen.New(pool)}
}

// Ping verifies database connectivity through the sqlc-generated query.
func (s *Store) Ping(ctx context.Context) error {
	_, err := s.q.Ping(ctx)
	return err
}
