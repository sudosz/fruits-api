package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sudosz/fruits-api/internal/middleware"
)

const testReapEvery = 10 * time.Minute

func newEngine(handlers ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(handlers...)
	router.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	router.POST("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return router
}

func TestSecurityHeaders(t *testing.T) {
	router := newEngine(middleware.SecurityHeaders())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	want := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Resource-Policy": "same-origin",
	}
	for header, value := range want {
		if got := rec.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("Content-Security-Policy is empty, want a policy")
	}
	if rec.Header().Get("Permissions-Policy") == "" {
		t.Error("Permissions-Policy is empty, want a policy")
	}
}

func TestBodyLimitRejectsOversizedBody(t *testing.T) {
	router := newEngine(middleware.BodyLimit(16))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 64)))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if !strings.Contains(rec.Body.String(), "request body too large") {
		t.Fatalf("body = %q, want request body too large", rec.Body.String())
	}
}

func TestBodyLimitAllowsSmallBody(t *testing.T) {
	router := newEngine(middleware.BodyLimit(1024))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"fruit":"apple"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRateLimitBlocksBurstOverflow(t *testing.T) {
	router := newEngine(middleware.RateLimit(1, 2, testReapEvery))

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want %q", got, "1")
	}
	if !strings.Contains(rec.Body.String(), "rate limit exceeded") {
		t.Fatalf("body = %q, want rate limit exceeded", rec.Body.String())
	}
}

func TestRateLimitIsPerClientIP(t *testing.T) {
	router := newEngine(middleware.RateLimit(1, 1, testReapEvery))

	first := httptest.NewRecorder()
	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.RemoteAddr = "192.0.2.20:1111"
	router.ServeHTTP(first, reqA)
	if first.Code != http.StatusOK {
		t.Fatalf("first client status = %d, want %d", first.Code, http.StatusOK)
	}

	second := httptest.NewRecorder()
	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.RemoteAddr = "192.0.2.21:2222"
	router.ServeHTTP(second, reqB)
	if second.Code != http.StatusOK {
		t.Fatalf("second client status = %d, want %d", second.Code, http.StatusOK)
	}

	third := httptest.NewRecorder()
	reqC := httptest.NewRequest(http.MethodGet, "/", nil)
	reqC.RemoteAddr = "192.0.2.20:1111"
	router.ServeHTTP(third, reqC)
	if third.Code != http.StatusTooManyRequests {
		t.Fatalf("repeat client status = %d, want %d", third.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimitDisabledWhenZero(t *testing.T) {
	router := newEngine(middleware.RateLimit(0, 0, testReapEvery))

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.0.2.30:3333"
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}
}
