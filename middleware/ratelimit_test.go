package middleware

import "testing"

func TestRateLimiterTokenBucket(t *testing.T) {
	rl := NewRateLimiter(60, 3)

	// Burst of 3 should succeed
	for i := 0; i < 3; i++ {
		ok, _ := rl.Allow("192.168.1.10")
		if !ok {
			t.Fatalf("expected request %d within burst to be allowed", i+1)
		}
	}

	// 4th immediate request should be throttled
	if ok, _ := rl.Allow("192.168.1.10"); ok {
		t.Fatalf("expected 4th immediate request to be rate-limited")
	}

	// Separate client IP should still be allowed
	if ok, _ := rl.Allow("10.0.0.5"); !ok {
		t.Fatalf("expected different IP to have independent token bucket")
	}
}
