package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if GetCurrentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	app.render(w, "login.html", &PageData{Title: "Sign In"})
}

func (app *App) handleGitHubAuth(w http.ResponseWriter, r *http.Request) {
	// Generate random state to prevent CSRF
	stateBytes := make([]byte, 16)
	rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/auth/callback",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(app.BaseURL, "https"),
		MaxAge:   600, // 10 minutes
	})

	redirectURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=read:user&state=%s",
		url.QueryEscape(app.GitHubClientID),
		url.QueryEscape(app.BaseURL+"/auth/callback"),
		url.QueryEscape(state),
	)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (app *App) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	// Verify CSRF state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" {
		app.renderStatus(w, r, http.StatusBadRequest, "Bad Request", "Missing OAuth state.")
		return
	}
	// Clear the state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/auth/callback",
		HttpOnly: true,
		MaxAge:   -1,
	})
	if r.URL.Query().Get("state") != stateCookie.Value {
		app.renderStatus(w, r, http.StatusBadRequest, "Bad Request", "Invalid OAuth state.")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		app.renderStatus(w, r, http.StatusBadRequest, "Bad Request", "Missing code parameter.")
		return
	}

	// Exchange code for access token
	token, err := app.exchangeGitHubCode(code)
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Authentication Error", "Failed to authenticate with GitHub.")
		return
	}

	// Get user info from GitHub
	ghUser, err := app.getGitHubUser(token)
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Authentication Error", "Failed to get GitHub user info.")
		return
	}

	// Upsert user in our DB
	user, err := app.UpsertUser(ghUser.ID, ghUser.Login, ghUser.Name, ghUser.AvatarURL)
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Authentication Error", "Failed to create user.")
		return
	}

	// Create session
	sessionToken, err := app.CreateSession(user.ID)
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Authentication Error", "Failed to create session.")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.HasPrefix(app.BaseURL, "https"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 60 * 60, // 30 days
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func (app *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		app.DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.HasPrefix(app.BaseURL, "https"),
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// --- GitHub API helpers ---

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

func (app *App) exchangeGitHubCode(code string) (string, error) {
	data := url.Values{
		"client_id":     {app.GitHubClientID},
		"client_secret": {app.GitHubClientSecret},
		"code":          {code},
	}

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token",
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp githubTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token")
	}

	return tokenResp.AccessToken, nil
}

func (app *App) getGitHubUser(accessToken string) (*githubUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var user githubUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}

	if user.Name == "" {
		user.Name = user.Login
	}

	return &user, nil
}
