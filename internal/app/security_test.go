package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithCSRFSetsCookieOnGet(t *testing.T) {
	app := &App{BaseURL: "https://devsocial.app"}
	handler := app.withCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	found := false
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == csrfCookieName && cookie.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("csrf cookie not set")
	}
}

func TestWithCSRFBLocksMissingToken(t *testing.T) {
	app := &App{Templates: LoadTemplates()}
	handler := app.withCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/posts", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "abc"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestWithCSRFAllowsMatchingHeader(t *testing.T) {
	app := &App{Templates: LoadTemplates()}
	handler := app.withCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/posts", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "abc"})
	req.Header.Set("X-CSRF-Token", "abc")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestWithSecurityHeadersSetsCSP(t *testing.T) {
	app := &App{}
	handler := app.withSecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") {
		t.Fatalf("unexpected CSP: %q", csp)
	}
}
