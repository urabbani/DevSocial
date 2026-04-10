package app

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type contextKey string

const userContextKey contextKey = "user"

// withUser middleware loads the current user from the session cookie into the request context.
func (app *App) withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err == nil && cookie.Value != "" {
			user, err := app.GetUserBySession(cookie.Value)
			if err == nil {
				ctx := context.WithValue(r.Context(), userContextKey, user)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// UserFromContext returns the logged-in user from the request context, or nil.
func UserFromContext(r *http.Request) *User {
	u, _ := r.Context().Value(userContextKey).(*User)
	return u
}

func (app *App) withRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.RateLimiter == nil {
			next.ServeHTTP(w, r)
			return
		}

		rule, ok := rateRuleForRequest(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		key := rule.Name + ":" + clientIP(r)
		allowed, retryAfter := app.RateLimiter.Allow(key, rule, time.Now())
		if allowed {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		http.Error(w, "Too many requests. Try again shortly.", http.StatusTooManyRequests)
	})
}
