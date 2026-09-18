package database

import (
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
func Connect(cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := ping(db, 10, 2*time.Second); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// Migrate creates the fruits table when it is missing.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create fruits table: %w", err)
	}
	return nil
}

func ping(db *sql.DB, attempts int, wait time.Duration) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = db.Ping(); err == nil {
			return nil
		}
		time.Sleep(wait)
	}
	return fmt.Errorf("postgres unreachable after %d attempts: %w", attempts, err)
}
