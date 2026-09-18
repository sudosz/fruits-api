package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/sudosz/fruits-api/internal/config"
)

const schema = `
CREATE TABLE IF NOT EXISTS fruits (
    id SERIAL PRIMARY KEY,
    fruit VARCHAR(100) NOT NULL,
    color VARCHAR(50) NOT NULL
);`

// Connect opens a pool against PostgreSQL and waits for it to answer. The
// retry loop exists because compose and Kubernetes can start the API before the
// database finishes accepting connections.
func Connect(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(time.Minute)

	if err := ping(ctx, db, 10, 2*time.Second); err != nil {
		// Closing a pool that never became usable cannot fail in a way the
		// caller could act on, so the original error is the one returned.
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("%w (close pool: %w)", err, closeErr)
		}
		return nil, err
	}

	return db, nil
}

// Migrate creates the fruits table when it is missing.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("create fruits table: %w", err)
	}
	return nil
}

func ping(ctx context.Context, db *sql.DB, attempts int, wait time.Duration) error {
	var err error
	for i := range attempts {
		if err = db.PingContext(ctx); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("postgres unreachable after %d attempts: %w", i+1, err)
		}

		// Sleep, but give up immediately if the caller cancels while waiting.
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("postgres unreachable after %d attempts: %w", i+1, err)
		case <-timer.C:
		}
	}
	return fmt.Errorf("postgres unreachable after %d attempts: %w", attempts, err)
}
