package app

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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
		if r.Header.Get("HX-Request") == "true" || strings.HasPrefix(r.URL.Path, "/api/") || strings.HasSuffix(r.URL.Path, ".md") || strings.HasSuffix(r.URL.Path, ".xml") {
			http.Error(w, "Rate limit exceeded. Try again shortly.", http.StatusTooManyRequests)
			return
		}

		app.renderStatus(w, r, http.StatusTooManyRequests, "Slow Down", "Too many requests from this IP. Try again shortly.")
	})
}

// GetCurrentUser returns the logged-in user from the request context, or nil.
func GetCurrentUser(r *http.Request) *User {
	u, _ := r.Context().Value(userContextKey).(*User)
	return u
}

// requireAuth wraps a handler to require authentication.
func (app *App) requireAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if GetCurrentUser(r) == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		handler(w, r)
	}
}
