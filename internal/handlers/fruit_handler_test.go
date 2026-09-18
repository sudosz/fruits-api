package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/sudosz/fruits-api/internal/handlers"
	"github.com/sudosz/fruits-api/internal/models"
	"github.com/sudosz/fruits-api/internal/repository"
	"github.com/sudosz/fruits-api/internal/service"
)

// stubService implements service.FruitService so the HTTP layer can be tested
// without a database.
type stubService struct {
	list       []models.Fruit
	listErr    error
	get        models.Fruit
	getErr     error
	created    models.Fruit
	createErr  error
	healthErr  error
	lastCreate models.CreateFruitRequest
}

func (s *stubService) List(context.Context) ([]models.Fruit, error) {
	return s.list, s.listErr
}

func (s *stubService) Get(_ context.Context, _ int) (models.Fruit, error) {
	return s.get, s.getErr
}

func (s *stubService) Create(_ context.Context, req models.CreateFruitRequest) (models.Fruit, error) {
	s.lastCreate = req
	return s.created, s.createErr
}

func (s *stubService) Health(context.Context) error {
	return s.healthErr
}

func newTestRouter(svc service.FruitService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.NewFruitHandler(svc).RegisterRoutes(router)
	return router
}

func do(t *testing.T, router *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	router := newTestRouter(&stubService{})

	rec := do(t, router, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got["status"] != "ok" {
		t.Fatalf("status field = %q, want %q", got["status"], "ok")
	}
}

func TestHealthzReportsUnavailableWhenDatabaseIsDown(t *testing.T) {
	router := newTestRouter(&stubService{healthErr: errors.New("ping failed")})

	rec := do(t, router, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "database unreachable") {
		t.Fatalf("body = %q, want database unreachable", rec.Body.String())
	}
}

func TestListReturnsEmptyArrayNotNull(t *testing.T) {
	router := newTestRouter(&stubService{list: []models.Fruit{}})

	rec := do(t, router, http.MethodGet, "/fruits", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Fatalf("body = %q, want []", body)
	}
}

func TestListReturnsFruits(t *testing.T) {
	router := newTestRouter(&stubService{list: []models.Fruit{
		{ID: 1, FruitAttributes: models.FruitAttributes{Fruit: "apple", Color: "red"}},
	}})

	rec := do(t, router, http.MethodGet, "/fruits", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got []models.Fruit
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(got) != 1 || got[0].Fruit != "apple" || got[0].Color != "red" || got[0].ID != 1 {
		t.Fatalf("got = %+v, want one apple/red with id 1", got)
	}
}

func TestGetRejectsInvalidID(t *testing.T) {
	router := newTestRouter(&stubService{})

	rec := do(t, router, http.MethodGet, "/fruits/abc", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid id") {
		t.Fatalf("body = %q, want invalid id", rec.Body.String())
	}
}

func TestGetReturnsNotFound(t *testing.T) {
	router := newTestRouter(&stubService{getErr: repository.ErrNotFound})

	rec := do(t, router, http.MethodGet, "/fruits/42", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if !strings.Contains(rec.Body.String(), "fruit not found") {
		t.Fatalf("body = %q, want fruit not found", rec.Body.String())
	}
}

func TestGetReturnsFruit(t *testing.T) {
	router := newTestRouter(&stubService{
		get: models.Fruit{ID: 7, FruitAttributes: models.FruitAttributes{Fruit: "banana", Color: "yellow"}},
	})

	rec := do(t, router, http.MethodGet, "/fruits/7", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got models.Fruit
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got.ID != 7 || got.Fruit != "banana" || got.Color != "yellow" {
		t.Fatalf("got = %+v, want id 7 banana/yellow", got)
	}
}

func TestCreateReturnsCreatedFruit(t *testing.T) {
	svc := &stubService{
		created: models.Fruit{ID: 3, FruitAttributes: models.FruitAttributes{Fruit: "kiwi", Color: "green"}},
	}
	router := newTestRouter(svc)

	rec := do(t, router, http.MethodPost, "/fruits", `{"fruit":"kiwi","color":"green"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if svc.lastCreate.Fruit != "kiwi" || svc.lastCreate.Color != "green" {
		t.Fatalf("service received %+v, want kiwi/green", svc.lastCreate)
	}

	var got models.Fruit
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got.ID != 3 {
		t.Fatalf("id = %d, want 3", got.ID)
	}
}

func TestCreateRejectsMissingFields(t *testing.T) {
	router := newTestRouter(&stubService{})

	rec := do(t, router, http.MethodPost, "/fruits", `{"fruit":"kiwi"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "fruit and color are required") {
		t.Fatalf("body = %q, want fruit and color are required", rec.Body.String())
	}
}

func TestCreateRejectsBlankFields(t *testing.T) {
	router := newTestRouter(&stubService{createErr: service.ErrInvalidInput})

	rec := do(t, router, http.MethodPost, "/fruits", `{"fruit":" ","color":" "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "fruit and color must not be empty") {
		t.Fatalf("body = %q, want fruit and color must not be empty", rec.Body.String())
	}
}

func TestCreateReportsRepositoryFailure(t *testing.T) {
	router := newTestRouter(&stubService{createErr: errors.New("insert failed")})

	rec := do(t, router, http.MethodPost, "/fruits", `{"fruit":"kiwi","color":"green"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "failed to create fruit") {
		t.Fatalf("body = %q, want failed to create fruit", rec.Body.String())
	}
}
