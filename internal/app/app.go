package app

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"devsocial/internal/claw"
	"devsocial/internal/ws"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type App struct {
	DB                 *sql.DB
	GitHubClientID     string
	GitHubClientSecret string
	BaseURL            string
	RateLimiter        *IPRateLimiter
	Hub                *ws.Hub
	Claw               *claw.Manager
}

func New(db *sql.DB, githubClientID, githubClientSecret, baseURL string) *App {
	binPath := os.Getenv("CLAW_BIN_PATH")
	if binPath == "" {
		binPath = claw.FindBinary()
	}
	dataDir := os.Getenv("CLAW_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	app := &App{
		DB:                 db,
		GitHubClientID:     githubClientID,
		GitHubClientSecret: githubClientSecret,
		BaseURL:            baseURL,
		RateLimiter:        NewIPRateLimiter(),
		Hub:                ws.NewHub(),
		Claw:               claw.NewManager(binPath, dataDir),
	}

	if claw.IsAvailable(binPath) {
		log.Printf("claw-code found at %s", binPath)
	} else {
		log.Printf("claw-code not found at %s — AI features disabled", binPath)
	}

	go app.Hub.Run()
	return app
}

func (app *App) Handler() http.Handler {
	mux := http.NewServeMux()

	// Static files + uploads
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(projectPath("static")))))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(projectPath("uploads")))))

	// React app — serve index.html for all non-API routes
	mux.HandleFunc("GET /", app.serveReactApp)

	// Auth
	mux.HandleFunc("GET /auth/github", app.handleGitHubAuth)
	mux.HandleFunc("GET /auth/callback", app.handleGitHubCallback)
	mux.HandleFunc("POST /auth/logout", app.requireAuth(app.handleLogout))

	// Current user
	mux.HandleFunc("GET /api/me", app.requireAuth(app.handleGetMe))

	// WebSocket
	mux.HandleFunc("GET /ws", app.requireAuth(app.handleWebSocket))

	// Workspaces
	mux.HandleFunc("GET /api/workspaces", app.requireAuth(app.handleListWorkspaces))
	mux.HandleFunc("POST /api/workspaces", app.requireAuth(app.handleCreateWorkspace))
	mux.HandleFunc("GET /api/workspaces/{id}", app.requireAuth(app.handleGetWorkspace))
	mux.HandleFunc("PATCH /api/workspaces/{id}", app.requireAuth(app.handleUpdateWorkspace))
	mux.HandleFunc("DELETE /api/workspaces/{id}", app.requireAuth(app.handleDeleteWorkspace))

	// Workspace members
	mux.HandleFunc("GET /api/workspaces/{id}/members", app.requireAuth(app.handleListMembers))
	mux.HandleFunc("POST /api/workspaces/{id}/members", app.requireAuth(app.handleAddMember))
	mux.HandleFunc("DELETE /api/workspaces/{id}/members/{uid}", app.requireAuth(app.handleRemoveMember))

	// Channels
	mux.HandleFunc("GET /api/workspaces/{id}/channels", app.requireAuth(app.handleListChannels))
	mux.HandleFunc("POST /api/workspaces/{id}/channels", app.requireAuth(app.handleCreateChannel))
	mux.HandleFunc("GET /api/channels/{id}", app.requireAuth(app.handleGetChannel))
	mux.HandleFunc("PATCH /api/channels/{id}", app.requireAuth(app.handleUpdateChannel))
	mux.HandleFunc("DELETE /api/channels/{id}", app.requireAuth(app.handleDeleteChannel))

	// Messages
	mux.HandleFunc("GET /api/channels/{id}/messages", app.requireAuth(app.handleListMessages))
	mux.HandleFunc("POST /api/channels/{id}/messages", app.requireAuth(app.handleCreateMessage))
	mux.HandleFunc("GET /api/messages/{id}", app.requireAuth(app.handleGetMessage))
	mux.HandleFunc("PATCH /api/messages/{id}", app.requireAuth(app.handleEditMessage))
	mux.HandleFunc("DELETE /api/messages/{id}", app.requireAuth(app.handleDeleteMessage))

	// Reactions
	mux.HandleFunc("POST /api/messages/{id}/reactions", app.requireAuth(app.handleToggleReaction))

	// AI Agents
	mux.HandleFunc("GET /api/workspaces/{id}/agents", app.requireAuth(app.handleListAgents))
	mux.HandleFunc("POST /api/workspaces/{id}/agents", app.requireAuth(app.handleCreateAgent))
	mux.HandleFunc("PATCH /api/agents/{id}", app.requireAuth(app.handleUpdateAgent))

	// Upload
	mux.HandleFunc("POST /upload", app.requireAuth(app.handleUpload))

	return app.withSecurityHeaders(app.withUser(app.withRateLimit(mux)))
}

func (app *App) serveReactApp(w http.ResponseWriter, r *http.Request) {
	// Serve the React SPA for all non-API, non-static routes
	// The React app handles client-side routing
	http.ServeFile(w, r, projectPath("web/dist/index.html"))
}

func (app *App) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r)
		if user == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
