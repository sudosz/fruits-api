// Package middleware holds cross-cutting HTTP middleware for the API.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets conservative response headers for a JSON-only API.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		// The API serves JSON plus the Swagger UI, which needs inline styles and
		// scripts from the same origin.
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		c.Next()
	}
}

// BodyLimit rejects oversized request bodies before a handler decodes them.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "request body too large",
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// RateLimit applies a per-client-IP token bucket. It is deliberately in-process:
// it protects a single replica from a noisy client. Cluster-wide limits belong
// at the ingress or gateway, which can see all replicas.
func RateLimit(ratePerSecond float64, burst int, reapEvery time.Duration) gin.HandlerFunc {
	if ratePerSecond <= 0 || burst <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	l := &ipLimiter{
		rate:    ratePerSecond,
		burst:   float64(burst),
		buckets: make(map[string]*bucket),
	}
	if reapEvery > 0 {
		go l.reap(reapEvery)
	}

	return func(c *gin.Context) {
		if !l.allow(c.ClientIP()) {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}

type bucket struct {
	tokens float64
	last   time.Time
}

type ipLimiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[string]*bucket
}

func (l *ipLimiter) allow(ip string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[ip]
	if !ok {
		l.buckets[ip] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}

	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// reap drops idle buckets so the map cannot grow without bound.
func (l *ipLimiter) reap(every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-every)
		l.mu.Lock()
		for ip, b := range l.buckets {
			if b.last.Before(cutoff) {
				delete(l.buckets, ip)
			}
		}
		l.mu.Unlock()
	}
}
