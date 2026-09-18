package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/sudosz/fruits-api/internal/config"
	"github.com/sudosz/fruits-api/internal/database"
	"github.com/sudosz/fruits-api/internal/models"
	"github.com/sudosz/fruits-api/internal/repository"
)

// openDB returns a live PostgreSQL pool, or skips the test when no database is
// configured (local runs without docker compose up).
func openDB(t *testing.T) *sql.DB {
	t.Helper()

	if os.Getenv("DB_HOST") == "" && os.Getenv("DATABASE_URL") == "" {
		t.Skip("no database configured: set DB_HOST or DATABASE_URL to run repository integration tests")
	}

	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg)
	if err != nil {
		t.Skipf("database unreachable: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	migrateCtx, cancelMigrate := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelMigrate()

	if err := database.Migrate(migrateCtx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return db
}

func TestRepositoryCreateThenGet(t *testing.T) {
	db := openDB(t)
	repo := repository.NewFruitRepository(db)
	ctx := context.Background()

	created, err := repo.Create(ctx, models.Fruit{
		FruitAttributes: models.FruitAttributes{Fruit: "testfruit", Color: "testcolor"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM fruits WHERE id = $1", created.ID)
	})

	if created.ID == 0 {
		t.Fatal("Create() returned zero id, want generated id")
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Fruit != "testfruit" || got.Color != "testcolor" || got.ID != created.ID {
		t.Fatalf("got = %+v, want %+v", got, created)
	}
}

func TestRepositoryGetMissingReturnsErrNotFound(t *testing.T) {
	db := openDB(t)
	repo := repository.NewFruitRepository(db)

	_, err := repo.Get(context.Background(), -1)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("err = %v, want repository.ErrNotFound", err)
	}
}

func TestRepositoryListReturnsNonNilSlice(t *testing.T) {
	db := openDB(t)
	repo := repository.NewFruitRepository(db)

	fruits, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if fruits == nil {
		t.Fatal("List() returned nil slice, want empty slice so JSON encodes []")
	}
}

func TestRepositoryHealth(t *testing.T) {
	db := openDB(t)

	if err := repository.NewFruitRepository(db).Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
}
