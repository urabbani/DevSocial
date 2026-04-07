package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

const csrfCookieName = "csrf_token"

type contextKey string

const (
	userContextKey contextKey = "user"
	csrfContextKey contextKey = "csrf"
)

func generateToken(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (app *App) withCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := app.ensureCSRFCookie(w, r)
		if token != "" {
			ctx := context.WithValue(r.Context(), csrfContextKey, token)
			r = r.WithContext(ctx)
		}

		if !csrfProtectedMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		if token == "" || !app.validCSRF(r, token) {
			app.renderCSRFError(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (app *App) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", strings.Join([]string{
			"default-src 'self'",
			"script-src 'self'",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' https: data:",
			"connect-src 'self'",
			"font-src 'self'",
			"object-src 'none'",
			"base-uri 'self'",
			"form-action 'self'",
			"frame-ancestors 'none'",
		}, "; "))
		next.ServeHTTP(w, r)
	})
}

func (app *App) ensureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	token, err := generateToken(32)
	if err != nil {
		return ""
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		Secure:   strings.HasPrefix(app.BaseURL, "https"),
		SameSite: http.SameSiteLaxMode,
	})
	return token
}

func csrfProtectedMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func (app *App) validCSRF(r *http.Request, token string) bool {
	headerToken := r.Header.Get("X-CSRF-Token")
	if headerToken != "" {
		return headerToken == token
	}
	if err := r.ParseForm(); err != nil {
		return false
	}
	return r.FormValue("csrf_token") == token
}

func (app *App) renderCSRFError(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" || strings.HasPrefix(r.URL.Path, "/api/") {
		http.Error(w, "Invalid CSRF token.", http.StatusForbidden)
		return
	}
	app.renderStatus(w, r, http.StatusForbidden, "Forbidden", "Invalid CSRF token.")
}
