package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

// RateLimiter implements a thread-safe per-IP token-bucket rate limiter
// with automatic background cleanup of idle client entries.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rate     float64 // tokens per second
	capacity float64 // max burst size
}

func NewRateLimiter(requestsPerMinute int, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     float64(requestsPerMinute) / 60.0,
		capacity: float64(burst),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) Allow(ip string) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[ip]
	if !exists {
		b = &tokenBucket{
			tokens:     rl.capacity,
			lastRefill: now,
			lastSeen:   now,
		}
		rl.buckets[ip] = b
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > rl.capacity {
		b.tokens = rl.capacity
	}
	b.lastRefill = now
	b.lastSeen = now

	if b.tokens < 1.0 {
		return false, 0
	}
	b.tokens -= 1.0
	return true, int(b.tokens)
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for ip, b := range rl.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never throttle static assets or health checks
		if strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		ip := extractClientIP(r)
		allowed, remaining := rl.Allow(ip)

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", rl.capacity))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			w.Header().Set("Retry-After", "10")
			if isAPIRequest(r) {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"Rate limit exceeded. Please retry shortly."}`, http.StatusTooManyRequests)
			} else {
				http.Error(w, "Too Many Requests — please wait a few seconds and retry.", http.StatusTooManyRequests)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
