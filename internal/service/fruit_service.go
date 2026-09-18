// Package service holds the business rules of the API. It depends on the
// repository interface, never on a database handle.
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/sudosz/fruits-api/internal/models"
	"github.com/sudosz/fruits-api/internal/repository"
)

// ErrNotFound is returned when the requested fruit does not exist.
var ErrNotFound = repository.ErrNotFound

// ErrInvalidInput is returned when a create payload carries empty attributes.
var ErrInvalidInput = errors.New("fruit and color must not be empty")

// FruitService is the business contract consumed by the HTTP layer.
type FruitService interface {
	List(ctx context.Context) ([]models.Fruit, error)
	Get(ctx context.Context, id int) (models.Fruit, error)
	Create(ctx context.Context, req models.CreateFruitRequest) (models.Fruit, error)
	Health(ctx context.Context) error
}

type fruitService struct {
	repo repository.FruitRepository
}

// NewFruitService returns a FruitService backed by the given repository.
func NewFruitService(repo repository.FruitRepository) FruitService {
	return &fruitService{repo: repo}
}

func (s *fruitService) List(ctx context.Context) ([]models.Fruit, error) {
	return s.repo.List(ctx)
}

func (s *fruitService) Get(ctx context.Context, id int) (models.Fruit, error) {
	return s.repo.Get(ctx, id)
}

func (s *fruitService) Create(ctx context.Context, req models.CreateFruitRequest) (models.Fruit, error) {
	name := strings.TrimSpace(req.Fruit)
	color := strings.TrimSpace(req.Color)
	if name == "" || color == "" {
		return models.Fruit{}, ErrInvalidInput
	}

	return s.repo.Create(ctx, models.Fruit{
		FruitAttributes: models.FruitAttributes{Fruit: name, Color: color},
	})
}

func (s *fruitService) Health(ctx context.Context) error {
	return s.repo.Health(ctx)
}
