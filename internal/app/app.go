package app

import (
	"database/sql"
	"log"
	"net/http"

	"devsocial/internal/ai"
	"devsocial/internal/rag"
	"devsocial/internal/storage"
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
	AI                 *ai.Provider
	Storage            *storage.MinIO
	RAG                *rag.Client
}

func New(db *sql.DB, githubClientID, githubClientSecret, baseURL string) *App {
	// AI provider (LiteLLM)
	aiProvider := ai.NewProvider()

	// Load model from settings
	var model string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = 'ai_model'`).Scan(&model)
	if err == nil && model != "" {
		aiProvider.SetModel(model)
		log.Printf("AI model from settings: %s", model)
	} else {
		aiProvider.SetModel("claude-sonnet")
		log.Printf("Using default AI model: claude-sonnet")
	}

	// File storage (MinIO)
	var store *storage.MinIO
	s, err := storage.NewMinIO()
	if err != nil {
		log.Printf("MinIO not available: %v — file features disabled", err)
	} else {
		store = s
		log.Printf("MinIO storage connected")
	}

	// RAG (ChromaDB)
	var ragClient *rag.Client
	rc := rag.NewClient()
	if err := rc.Health(); err != nil {
		log.Printf("ChromaDB not available: %v — RAG features disabled", err)
	} else {
		rc.EnsureCollection("devsocial_docs")
		ragClient = rc
		log.Printf("ChromaDB connected")
	}

	app := &App{
		DB:                 db,
		GitHubClientID:     githubClientID,
		GitHubClientSecret: githubClientSecret,
		BaseURL:            baseURL,
		RateLimiter:        NewIPRateLimiter(),
		Hub:                ws.NewHub(),
		AI:                 aiProvider,
		Storage:            store,
		RAG:                ragClient,
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

	// Health check (no auth required)
	mux.HandleFunc("GET /api/health", app.handleHealthCheck)

	// Admin settings
	mux.HandleFunc("GET /api/admin/settings", app.requireAuth(app.handleGetSettings))
	mux.HandleFunc("PATCH /api/admin/settings", app.requireAuth(app.handleUpdateSettings))
	mux.HandleFunc("GET /api/admin/models", app.requireAuth(app.handleGetModels))

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

	// Files
	mux.HandleFunc("POST /upload", app.requireAuth(app.handleUpload))
	mux.HandleFunc("GET /api/workspaces/{id}/files", app.requireAuth(app.handleListFiles))
	mux.HandleFunc("GET /api/files/{id}", app.requireAuth(app.handleGetFile))
	mux.HandleFunc("DELETE /api/files/{id}", app.requireAuth(app.handleDeleteFile))

	// Feed / Posts
	mux.HandleFunc("GET /api/workspaces/{id}/feed", app.requireAuth(app.handleGetFeed))
	mux.HandleFunc("POST /api/workspaces/{id}/feed", app.requireAuth(app.handleCreatePost))
	mux.HandleFunc("GET /api/posts/{id}", app.requireAuth(app.handleGetPost))
	mux.HandleFunc("POST /api/posts/{id}/reactions", app.requireAuth(app.handleTogglePostReaction))
	mux.HandleFunc("GET /api/posts/{id}/replies", app.requireAuth(app.handleGetPostReplies))
	mux.HandleFunc("DELETE /api/posts/{id}", app.requireAuth(app.handleDeletePost))

	// Tasks
	mux.HandleFunc("GET /api/workspaces/{id}/tasks", app.requireAuth(app.handleListTasks))
	mux.HandleFunc("POST /api/workspaces/{id}/tasks", app.requireAuth(app.handleCreateTask))
	mux.HandleFunc("PATCH /api/tasks/{id}", app.requireAuth(app.handleUpdateTask))
	mux.HandleFunc("DELETE /api/tasks/{id}", app.requireAuth(app.handleDeleteTask))

	// Search
	mux.HandleFunc("GET /api/search", app.requireAuth(app.handleSearch))

	return app.withSecurityHeaders(app.withUser(app.withRateLimit(mux)))
}

func (app *App) serveReactApp(w http.ResponseWriter, r *http.Request) {
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
