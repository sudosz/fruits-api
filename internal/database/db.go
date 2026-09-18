package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "github.com/lib/pq"

	"github.com/sudosz/fruits-api/internal/config"
	"github.com/sudosz/fruits-api/migrations"
)

// Connect opens a pool against PostgreSQL, applies the pool limits from config
// and verifies connectivity once, bounded by the caller's context.
func Connect(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)

	if err := db.PingContext(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("ping postgres: %w (close pool: %w)", err, closeErr)
		}
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

// Migrate applies every pending up migration from the embedded migrations
// package, recording applied versions in schema_migrations.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	pending, err := upMigrations()
	if err != nil {
		return err
	}

	for _, m := range pending {
		if applied[m.version] {
			continue
		}
		if err := apply(ctx, db, m); err != nil {
			return err
		}
	}

	return nil
}

type migration struct {
	version int64
	name    string
	sql     string
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[int64]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	applied := make(map[int64]bool)
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	return applied, nil
}

func upMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	out := make([]migration, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		version, err := parseVersion(name)
		if err != nil {
			return nil, err
		}

		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}

		out = append(out, migration{version: version, name: name, sql: string(body)})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func parseVersion(name string) (int64, error) {
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("migration %s: expected <version>_<name>.up.sql", name)
	}

	var version int64
	if _, err := fmt.Sscanf(prefix, "%d", &version); err != nil {
		return 0, fmt.Errorf("migration %s: parse version: %w", name, err)
	}
	return version, nil
}

func apply(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", m.name, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("apply migration %s: %w", m.name, err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING", m.version,
	); err != nil {
		return fmt.Errorf("record migration %s: %w", m.name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", m.name, err)
	}
	return nil
}
