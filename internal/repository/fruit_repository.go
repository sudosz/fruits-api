// Package repository owns all SQL access for the API. Callers depend on the
// FruitRepository interface, never on a concrete driver or *sql.DB.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/sudosz/fruits-api/internal/models"
)

// ErrNotFound is returned when a requested fruit does not exist.
var ErrNotFound = errors.New("fruit not found")

// FruitRepository is the persistence contract for fruits.
type FruitRepository interface {
	List(ctx context.Context) ([]models.Fruit, error)
	Get(ctx context.Context, id int) (models.Fruit, error)
	Create(ctx context.Context, fruit models.Fruit) (models.Fruit, error)
	Health(ctx context.Context) error
}

type postgresRepository struct {
	db *sql.DB
}

// NewFruitRepository returns a PostgreSQL-backed FruitRepository.
func NewFruitRepository(db *sql.DB) FruitRepository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) List(ctx context.Context) ([]models.Fruit, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, fruit, color FROM fruits ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("query fruits: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Non-nil slice so an empty table marshals to [] rather than null.
	fruits := make([]models.Fruit, 0)
	for rows.Next() {
		var f models.Fruit
		if err := rows.Scan(&f.ID, &f.Fruit, &f.Color); err != nil {
			return nil, fmt.Errorf("scan fruit: %w", err)
		}
		fruits = append(fruits, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fruits: %w", err)
	}

	return fruits, nil
}

func (r *postgresRepository) Get(ctx context.Context, id int) (models.Fruit, error) {
	var f models.Fruit
	err := r.db.QueryRowContext(ctx,
		"SELECT id, fruit, color FROM fruits WHERE id = $1", id,
	).Scan(&f.ID, &f.Fruit, &f.Color)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return models.Fruit{}, ErrNotFound
	case err != nil:
		return models.Fruit{}, fmt.Errorf("query fruit %d: %w", id, err)
	}
	return f, nil
}

func (r *postgresRepository) Create(ctx context.Context, fruit models.Fruit) (models.Fruit, error) {
	err := r.db.QueryRowContext(ctx,
		"INSERT INTO fruits (fruit, color) VALUES ($1, $2) RETURNING id",
		fruit.Fruit, fruit.Color,
	).Scan(&fruit.ID)
	if err != nil {
		return models.Fruit{}, fmt.Errorf("insert fruit: %w", err)
	}
	return fruit, nil
}

func (r *postgresRepository) Health(ctx context.Context) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}
