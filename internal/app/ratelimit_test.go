package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIPRateLimiterBlocksBurst(t *testing.T) {
	rl := NewIPRateLimiter()
	rule := rateRule{Name: "read", Rate: 1, Burst: 2}
	now := time.Now()

	if ok, _ := rl.Allow("read:127.0.0.1", rule, now); !ok {
		t.Fatal("first request should pass")
	}
	if ok, _ := rl.Allow("read:127.0.0.1", rule, now); !ok {
		t.Fatal("second request should pass")
	}
	if ok, retryAfter := rl.Allow("read:127.0.0.1", rule, now); ok || retryAfter < 1 {
		t.Fatalf("third request = ok %v retryAfter %d, want blocked with retry", ok, retryAfter)
	}
}

func TestWithRateLimitSkipsStatic(t *testing.T) {
	app := &App{RateLimiter: NewIPRateLimiter()}
	handler := app.withRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestWithRateLimitReturns429ForAPI(t *testing.T) {
	app := &App{RateLimiter: NewIPRateLimiter()}
	handler := app.withRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < 60; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/users/karpathy/posts", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("request %d status = %d, want %d", i+1, rr.Code, http.StatusNoContent)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users/karpathy/posts", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusTooManyRequests)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header missing")
	}
}
