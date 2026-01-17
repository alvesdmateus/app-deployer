package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool
	RequestsPerSecond float64
	BurstSize         int
	CleanupInterval   time.Duration
}

// DefaultRateLimitConfig returns sensible defaults for rate limiting
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 10.0, // 10 requests per second
		BurstSize:         20,   // Allow bursts up to 20 requests
		CleanupInterval:   5 * time.Minute,
	}
}

// IPRateLimiter manages rate limiters per IP address
type IPRateLimiter struct {
	limiters map[string]*clientLimiter
	mu       sync.RWMutex
	config   RateLimitConfig
	stopCh   chan struct{}
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter creates a new IP-based rate limiter
func NewIPRateLimiter(config RateLimitConfig) *IPRateLimiter {
	rl := &IPRateLimiter{
		limiters: make(map[string]*clientLimiter),
		config:   config,
		stopCh:   make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// GetLimiter returns the rate limiter for a given IP
func (rl *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if cl, exists := rl.limiters[ip]; exists {
		cl.lastSeen = time.Now()
		return cl.limiter
	}

	limiter := rate.NewLimiter(rate.Limit(rl.config.RequestsPerSecond), rl.config.BurstSize)
	rl.limiters[ip] = &clientLimiter{
		limiter:  limiter,
		lastSeen: time.Now(),
	}

	return limiter
}

// cleanupLoop periodically removes old entries
func (rl *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCh:
			return
		}
	}
}

// cleanup removes stale limiters
func (rl *IPRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	threshold := time.Now().Add(-rl.config.CleanupInterval)
	for ip, cl := range rl.limiters {
		if cl.lastSeen.Before(threshold) {
			delete(rl.limiters, ip)
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *IPRateLimiter) Stop() {
	close(rl.stopCh)
}

// RateLimitMiddleware creates a middleware that limits requests per IP
func RateLimitMiddleware(config RateLimitConfig) func(http.Handler) http.Handler {
	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	limiter := NewIPRateLimiter(config)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			if !limiter.GetLimiter(ip).Allow() {
				log.Warn().
					Str("ip", ip).
					Str("path", r.URL.Path).
					Msg("Rate limit exceeded")

				w.Header().Set("Retry-After", "1")
				RespondWithError(w, http.StatusTooManyRequests, "Rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// EndpointRateLimiter provides different rate limits for different endpoints
type EndpointRateLimiter struct {
	defaultConfig RateLimitConfig
	endpoints     map[string]RateLimitConfig
	limiters      map[string]*IPRateLimiter
	mu            sync.RWMutex
}

// NewEndpointRateLimiter creates a rate limiter with per-endpoint configuration
func NewEndpointRateLimiter(defaultConfig RateLimitConfig) *EndpointRateLimiter {
	return &EndpointRateLimiter{
		defaultConfig: defaultConfig,
		endpoints:     make(map[string]RateLimitConfig),
		limiters:      make(map[string]*IPRateLimiter),
	}
}

// SetEndpointLimit sets a custom limit for a specific endpoint pattern
func (el *EndpointRateLimiter) SetEndpointLimit(pattern string, config RateLimitConfig) {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.endpoints[pattern] = config
}

// getLimiter returns the appropriate limiter for an endpoint
func (el *EndpointRateLimiter) getLimiter(path string) *IPRateLimiter {
	el.mu.RLock()
	config, exists := el.endpoints[path]
	el.mu.RUnlock()

	if !exists {
		config = el.defaultConfig
	}

	el.mu.Lock()
	defer el.mu.Unlock()

	key := path
	if !exists {
		key = "default"
	}

	if limiter, ok := el.limiters[key]; ok {
		return limiter
	}

	limiter := NewIPRateLimiter(config)
	el.limiters[key] = limiter
	return limiter
}

// Middleware returns the rate limiting middleware
func (el *EndpointRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			limiter := el.getLimiter(r.URL.Path)

			if !limiter.GetLimiter(ip).Allow() {
				log.Warn().
					Str("ip", ip).
					Str("path", r.URL.Path).
					Msg("Rate limit exceeded")

				w.Header().Set("Retry-After", "1")
				RespondWithError(w, http.StatusTooManyRequests, "Rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Stop stops all limiter cleanup goroutines
func (el *EndpointRateLimiter) Stop() {
	el.mu.Lock()
	defer el.mu.Unlock()

	for _, limiter := range el.limiters {
		limiter.Stop()
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (set by reverse proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		return splitFirst(xff, ",")
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return splitFirst(r.RemoteAddr, ":")
}

// splitFirst splits a string and returns the first part
func splitFirst(s, sep string) string {
	for i := 0; i < len(s); i++ {
		if s[i:i+1] == sep {
			return s[:i]
		}
	}
	return s
}

// AuthRateLimitConfig returns stricter limits for auth endpoints
func AuthRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 1.0, // 1 request per second
		BurstSize:         5,   // Allow bursts up to 5 requests
		CleanupInterval:   5 * time.Minute,
	}
}

// BuildRateLimitConfig returns limits suitable for build operations
func BuildRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 0.5, // 1 request per 2 seconds
		BurstSize:         3,   // Allow bursts up to 3 requests
		CleanupInterval:   5 * time.Minute,
	}
}
