package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sudosz/fruits-api/internal/models"
	"github.com/sudosz/fruits-api/internal/repository"
	"github.com/sudosz/fruits-api/internal/service"
)

// stubRepo implements repository.FruitRepository in memory.
type stubRepo struct {
	list      []models.Fruit
	listErr   error
	get       models.Fruit
	getErr    error
	created   models.Fruit
	createErr error
	healthErr error

	lastCreate models.Fruit
	lastGetID  int
}

func (r *stubRepo) List(context.Context) ([]models.Fruit, error) {
	return r.list, r.listErr
}

func (r *stubRepo) Get(_ context.Context, id int) (models.Fruit, error) {
	r.lastGetID = id
	return r.get, r.getErr
}

func (r *stubRepo) Create(_ context.Context, fruit models.Fruit) (models.Fruit, error) {
	r.lastCreate = fruit
	if r.createErr != nil {
		return models.Fruit{}, r.createErr
	}
	if r.created.Fruit == "" {
		fruit.ID = 1
		return fruit, nil
	}
	return r.created, nil
}

func (r *stubRepo) Health(context.Context) error {
	return r.healthErr
}

func req(fruit, color string) models.CreateFruitRequest {
	return models.CreateFruitRequest{
		FruitAttributes: models.FruitAttributes{Fruit: fruit, Color: color},
	}
}

func TestListPassesThroughRepositoryRows(t *testing.T) {
	repo := &stubRepo{list: []models.Fruit{
		{ID: 1, FruitAttributes: models.FruitAttributes{Fruit: "apple", Color: "red"}},
	}}

	got, err := service.NewFruitService(repo).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].Fruit != "apple" {
		t.Fatalf("got = %+v, want one apple", got)
	}
}

func TestListPropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("query failed")
	repo := &stubRepo{listErr: wantErr}

	if _, err := service.NewFruitService(repo).List(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestGetForwardsIDAndMapsNotFound(t *testing.T) {
	repo := &stubRepo{getErr: repository.ErrNotFound}

	_, err := service.NewFruitService(repo).Get(context.Background(), 99)
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("err = %v, want service.ErrNotFound", err)
	}
	if repo.lastGetID != 99 {
		t.Fatalf("repository received id %d, want 99", repo.lastGetID)
	}
}

func TestCreateTrimsWhitespaceBeforePersisting(t *testing.T) {
	repo := &stubRepo{}

	got, err := service.NewFruitService(repo).Create(context.Background(), req("  kiwi ", " green "))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repo.lastCreate.Fruit != "kiwi" || repo.lastCreate.Color != "green" {
		t.Fatalf("repository received %+v, want kiwi/green", repo.lastCreate)
	}
	if got.ID != 1 {
		t.Fatalf("id = %d, want 1", got.ID)
	}
}

func TestCreateRejectsEmptyAttributes(t *testing.T) {
	cases := map[string]models.CreateFruitRequest{
		"both blank":  req("   ", "   "),
		"blank fruit": req("  ", "red"),
		"blank color": req("apple", " "),
	}

	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &stubRepo{}

			_, err := service.NewFruitService(repo).Create(context.Background(), payload)
			if !errors.Is(err, service.ErrInvalidInput) {
				t.Fatalf("err = %v, want service.ErrInvalidInput", err)
			}
			if repo.lastCreate.Fruit != "" || repo.lastCreate.Color != "" {
				t.Fatalf("repository was called with %+v, want no call", repo.lastCreate)
			}
		})
	}
}

func TestHealthPropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("ping failed")
	repo := &stubRepo{healthErr: wantErr}

	if err := service.NewFruitService(repo).Health(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestHealthSucceedsWhenRepositoryIsHealthy(t *testing.T) {
	if err := service.NewFruitService(&stubRepo{}).Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
}
