package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"expense-matrix/services"
)

const RequestIDKey contextKey = "requestID"

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// SecurityAndObservability attaches Request-ID tracing, security headers, panic recovery,
// structured latency logging, and real-time telemetry metrics.
func SecurityAndObservability(metrics *services.MetricsService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = newRequestID()
			}

			// Hardened HTTP Security Headers
			w.Header().Set("X-Request-ID", reqID)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			defer func() {
				if recErr := recover(); recErr != nil {
					log.Printf(`{"level":"error","req_id":"%s","path":"%s","panic":"%v","stack":"%s"}`, reqID, r.URL.Path, recErr, strings.ReplaceAll(string(debug.Stack()), "\n", " | "))
					if metrics != nil {
						metrics.RecordRequest(http.StatusInternalServerError, time.Since(start))
					}
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
			next.ServeHTTP(rec, r.WithContext(ctx))

			dur := time.Since(start)
			if metrics != nil && !strings.HasPrefix(r.URL.Path, "/static/") {
				metrics.RecordRequest(rec.status, dur)
			}

			if !strings.HasPrefix(r.URL.Path, "/static/") && r.URL.Path != "/health" {
				log.Printf(`{"req_id":"%s","method":"%s","path":"%s","status":%d,"latency_ms":%.2f}`,
					reqID, r.Method, r.URL.Path, rec.status, float64(dur.Microseconds())/1000.0)
			}
		})
	}
}
