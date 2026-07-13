package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for goose
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Migrate applies all pending goose migrations from migrationsFS using a
// short-lived database/sql connection. It returns how many were applied.
func Migrate(ctx context.Context, databaseURL string, migrationsFS fs.FS) (int, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return 0, fmt.Errorf("open migration connection: %w", err)
	}
	defer db.Close()

	// Advisory session lock: zero-downtime redeploys briefly run two
	// instances, and both boot through this path.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return 0, fmt.Errorf("create migration session locker: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationsFS,
		goose.WithSessionLocker(locker))
	if err != nil {
		return 0, fmt.Errorf("create migration provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return 0, fmt.Errorf("apply migrations: %w", err)
	}
	return len(results), nil
}
