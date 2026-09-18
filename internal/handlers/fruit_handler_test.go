package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sudosz/fruits-api/internal/config"
	"github.com/sudosz/fruits-api/internal/database"
	"github.com/sudosz/fruits-api/internal/handlers"
	"github.com/sudosz/fruits-api/internal/models"
)

// setup returns a router wired to a real PostgreSQL instance. The tests skip
// instead of failing when no database is reachable, so `go test ./...` stays
// usable on a laptop without Postgres running while CI still exercises them.
func setup(t *testing.T) (*gin.Engine, *sql.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)

	db, err := sql.Open("postgres", config.Load().DSN())
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("close database: %v", err)
		}
	})

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.ExecContext(ctx, "TRUNCATE fruits RESTART IDENTITY"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	router := gin.New()
	handlers.New(db).RegisterRoutes(router)
	return router, db
}

func do(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestCreateFruit(t *testing.T) {
	router, _ := setup(t)

	rec := do(t, router, http.MethodPost, "/fruits", `{"fruit":"apple","color":"red"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}

	var got models.Fruit
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID == 0 {
		t.Error("id = 0, want generated id")
	}
	if got.Fruit != "apple" || got.Color != "red" {
		t.Errorf("got %+v, want apple/red", got)
	}
}

func TestListFruits(t *testing.T) {
	router, _ := setup(t)

	rec := do(t, router, http.MethodGet, "/fruits", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "[]" {
		t.Errorf("empty table body = %s, want []", body)
	}

	do(t, router, http.MethodPost, "/fruits", `{"fruit":"banana","color":"yellow"}`)

	rec = do(t, router, http.MethodGet, "/fruits", "")
	var fruits []models.Fruit
	if err := json.Unmarshal(rec.Body.Bytes(), &fruits); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(fruits) != 1 || fruits[0].Fruit != "banana" {
		t.Fatalf("got %+v, want one banana", fruits)
	}
}

func TestGetFruitByID(t *testing.T) {
	router, _ := setup(t)

	rec := do(t, router, http.MethodPost, "/fruits", `{"fruit":"kiwi","color":"green"}`)
	var created models.Fruit
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	rec = do(t, router, http.MethodGet, "/fruits/"+itoa(created.ID), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got models.Fruit
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got != created {
		t.Errorf("got %+v, want %+v", got, created)
	}
}

func TestGetFruitNotFound(t *testing.T) {
	router, _ := setup(t)

	rec := do(t, router, http.MethodGet, "/fruits/424242", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body models.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error != "fruit not found" {
		t.Errorf("error = %q, want \"fruit not found\"", body.Error)
	}
}

func TestGetFruitInvalidID(t *testing.T) {
	router, _ := setup(t)

	rec := do(t, router, http.MethodGet, "/fruits/not-a-number", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateFruitBadPayloads(t *testing.T) {
	router, _ := setup(t)

	cases := map[string]string{
		"malformed json": `{"fruit":`,
		"missing color":  `{"fruit":"apple"}`,
		"missing fruit":  `{"color":"red"}`,
		"empty values":   `{"fruit":"","color":""}`,
		"blank values":   `{"fruit":"  ","color":"  "}`,
		"empty body":     ``,
	}

	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			rec := do(t, router, http.MethodPost, "/fruits", payload)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	router, _ := setup(t)

	rec := do(t, router, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body models.HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want \"ok\"", body.Status)
	}
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
