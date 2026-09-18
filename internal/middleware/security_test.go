package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/sudosz/fruits-api/internal/middleware"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func newEngine(mw ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(mw...)
	r.POST("/echo", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

// request builds a test request bound to the test's context and, when given,
// pins the client IP so the per-IP rate limiter can be exercised.
func request(t *testing.T, method, path, body, remoteAddr string) *http.Request {
	t.Helper()

	var req *http.Request
	if body == "" {
		req = httptest.NewRequestWithContext(t.Context(), method, path, nil)
	} else {
		req = httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	return req
}

func TestSecurityHeaders(t *testing.T) {
	r := newEngine(middleware.SecurityHeaders())

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, request(t, http.MethodGet, "/ping", "", ""))

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	}
	for header, value := range want {
		if got := rec.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("Content-Security-Policy is missing")
	}
}

func TestBodyLimitRejectsOversizedBody(t *testing.T) {
	r := newEngine(middleware.BodyLimit(16))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, request(t, http.MethodPost, "/echo", strings.Repeat("a", 64), ""))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestBodyLimitAllowsSmallBody(t *testing.T) {
	r := newEngine(middleware.BodyLimit(1024))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, request(t, http.MethodPost, "/echo", `{"fruit":"apple"}`, ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRateLimitBlocksBurstOverflow(t *testing.T) {
	// One token per second with a burst of two: the third immediate request
	// has no tokens left.
	r := newEngine(middleware.RateLimit(1, 2))

	codes := make([]int, 0, 3)
	for range 3 {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, request(t, http.MethodGet, "/ping", "", "192.0.2.10:1234"))
		codes = append(codes, rec.Code)
	}

	if codes[0] != http.StatusOK || codes[1] != http.StatusOK {
		t.Fatalf("first two requests = %v, want both 200", codes[:2])
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Fatalf("third request = %d, want %d", codes[2], http.StatusTooManyRequests)
	}
	if retry := "Retry-After"; codes[2] == http.StatusTooManyRequests {
		// The throttled response must tell the client when to come back.
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, request(t, http.MethodGet, "/ping", "", "192.0.2.10:1234"))
		if rec.Header().Get(retry) == "" {
			t.Errorf("%s header is missing on a throttled response", retry)
		}
	}
}

func TestRateLimitIsPerClientIP(t *testing.T) {
	r := newEngine(middleware.RateLimit(1, 1))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, request(t, http.MethodGet, "/ping", "", "192.0.2.11:1234"))
	if rec.Code != http.StatusOK {
		t.Fatalf("first client = %d, want 200", rec.Code)
	}

	// A different IP has its own bucket and must not be throttled.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, request(t, http.MethodGet, "/ping", "", "192.0.2.12:1234"))
	if rec.Code != http.StatusOK {
		t.Fatalf("second client = %d, want 200", rec.Code)
	}
}

func TestRateLimitDisabledWhenZero(t *testing.T) {
	r := newEngine(middleware.RateLimit(0, 0))

	for i := range 5 {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, request(t, http.MethodGet, "/ping", "", "192.0.2.13:1234"))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d = %d, want 200", i, rec.Code)
		}
	}
}
