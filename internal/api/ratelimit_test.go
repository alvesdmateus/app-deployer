package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 10.0,
			BurstSize:         10,
			CleanupInterval:   1 * time.Minute,
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := RateLimitMiddleware(config)
		wrapped := middleware(handler)

		// Should allow first 10 requests (burst size)
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "request %d should be allowed", i+1)
		}
	})

	t.Run("blocks requests exceeding limit", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 1.0,
			BurstSize:         2,
			CleanupInterval:   1 * time.Minute,
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := RateLimitMiddleware(config)
		wrapped := middleware(handler)

		// First 2 requests should pass (burst)
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "192.168.1.2:12345"
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		}

		// Third request should be rate limited
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.Equal(t, "1", rec.Header().Get("Retry-After"))
	})

	t.Run("different IPs have separate limits", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 1.0,
			BurstSize:         1,
			CleanupInterval:   1 * time.Minute,
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := RateLimitMiddleware(config)
		wrapped := middleware(handler)

		// IP 1 uses their quota
		req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req1.RemoteAddr = "192.168.1.10:12345"
		rec1 := httptest.NewRecorder()
		wrapped.ServeHTTP(rec1, req1)
		assert.Equal(t, http.StatusOK, rec1.Code)

		// IP 1 should be rate limited
		req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req2.RemoteAddr = "192.168.1.10:12345"
		rec2 := httptest.NewRecorder()
		wrapped.ServeHTTP(rec2, req2)
		assert.Equal(t, http.StatusTooManyRequests, rec2.Code)

		// IP 2 should still be allowed
		req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req3.RemoteAddr = "192.168.1.20:12345"
		rec3 := httptest.NewRecorder()
		wrapped.ServeHTTP(rec3, req3)
		assert.Equal(t, http.StatusOK, rec3.Code)
	})

	t.Run("disabled rate limiting passes all requests", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           false,
			RequestsPerSecond: 1.0,
			BurstSize:         1,
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := RateLimitMiddleware(config)
		wrapped := middleware(handler)

		// All requests should pass when disabled
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "192.168.1.30:12345"
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})

	t.Run("respects X-Forwarded-For header", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 1.0,
			BurstSize:         1,
			CleanupInterval:   1 * time.Minute,
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := RateLimitMiddleware(config)
		wrapped := middleware(handler)

		// Use real client IP from X-Forwarded-For
		req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req1.RemoteAddr = "10.0.0.1:12345" // Proxy IP
		req1.Header.Set("X-Forwarded-For", "203.0.113.50")
		rec1 := httptest.NewRecorder()
		wrapped.ServeHTTP(rec1, req1)
		assert.Equal(t, http.StatusOK, rec1.Code)

		// Second request from same client IP should be rate limited
		req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req2.RemoteAddr = "10.0.0.1:12345"
		req2.Header.Set("X-Forwarded-For", "203.0.113.50")
		rec2 := httptest.NewRecorder()
		wrapped.ServeHTTP(rec2, req2)
		assert.Equal(t, http.StatusTooManyRequests, rec2.Code)

		// Different client through same proxy should be allowed
		req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req3.RemoteAddr = "10.0.0.1:12345"
		req3.Header.Set("X-Forwarded-For", "203.0.113.100")
		rec3 := httptest.NewRecorder()
		wrapped.ServeHTTP(rec3, req3)
		assert.Equal(t, http.StatusOK, rec3.Code)
	})
}

func TestIPRateLimiter(t *testing.T) {
	t.Run("creates and retrieves limiter", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 5.0,
			BurstSize:         10,
			CleanupInterval:   1 * time.Minute,
		}

		rl := NewIPRateLimiter(config)
		defer rl.Stop()

		limiter1 := rl.GetLimiter("192.168.1.1")
		limiter2 := rl.GetLimiter("192.168.1.1")

		// Should return same limiter for same IP
		assert.Same(t, limiter1, limiter2)

		// Should return different limiter for different IP
		limiter3 := rl.GetLimiter("192.168.1.2")
		assert.NotSame(t, limiter1, limiter3)
	})

	t.Run("concurrent access is safe", func(t *testing.T) {
		config := RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 100.0,
			BurstSize:         100,
			CleanupInterval:   1 * time.Minute,
		}

		rl := NewIPRateLimiter(config)
		defer rl.Stop()

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				ip := "192.168.1.1"
				limiter := rl.GetLimiter(ip)
				limiter.Allow()
			}(i)
		}
		wg.Wait()
	})
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		xForwardedFor  string
		xRealIP        string
		expectedIP     string
	}{
		{
			name:       "uses RemoteAddr when no headers",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:          "prefers X-Forwarded-For",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.50",
			expectedIP:    "203.0.113.50",
		},
		{
			name:          "takes first IP from X-Forwarded-For chain",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.50,10.0.0.1,192.168.1.1",
			expectedIP:    "203.0.113.50",
		},
		{
			name:       "uses X-Real-IP when no X-Forwarded-For",
			remoteAddr: "10.0.0.1:12345",
			xRealIP:    "203.0.113.60",
			expectedIP: "203.0.113.60",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			ip := getClientIP(req)
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

func TestAuthRateLimitConfig(t *testing.T) {
	config := AuthRateLimitConfig()
	assert.True(t, config.Enabled)
	assert.Equal(t, 1.0, config.RequestsPerSecond)
	assert.Equal(t, 5, config.BurstSize)
}

func TestBuildRateLimitConfig(t *testing.T) {
	config := BuildRateLimitConfig()
	assert.True(t, config.Enabled)
	assert.Equal(t, 0.5, config.RequestsPerSecond)
	assert.Equal(t, 3, config.BurstSize)
}
