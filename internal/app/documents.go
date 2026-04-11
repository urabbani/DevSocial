package app

import (
	"devsocial/internal/ai"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// --- Document Models ---

type CodeDocument struct {
	ID              int64     `json:"id"`
	WorkspaceID     int64     `json:"workspace_id"`
	Title           string    `json:"title"`
	Filename        string    `json:"filename"`
	Language        string    `json:"language"`
	Content         string    `json:"content"`
	Version         int       `json:"version"`
	CreatedBy       int64     `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	LastEditedBy    *int64    `json:"last_edited_by,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedByName   string    `json:"created_by_name,omitempty"`
	LastEditedByName string  `json:"last_edited_by_name,omitempty"`
}

const maxDocumentContent = 1_000_000 // 1MB limit

// --- Document Handlers ---

func (app *App) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	workspaceID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	if ok, _ := app.IsWorkspaceMember(workspaceID, user.ID); !ok {
		writeError(w, http.StatusForbidden, "not a member of this workspace")
		return
	}

	// Exclude content column from list for performance, add limit
	q := `SELECT d.id, d.workspace_id, d.title, d.filename, d.language, '' as content, d.version,
	             d.created_by, d.last_edited_by, d.created_at, d.updated_at,
	             cu.username, COALESCE(leu.username, '')
	      FROM code_documents d
	      JOIN users cu ON cu.id = d.created_by
	      LEFT JOIN users leu ON leu.id = d.last_edited_by
	      WHERE d.workspace_id = $1
	      ORDER BY d.updated_at DESC
	      LIMIT 200`

	docs, err := app.scanDocuments(q, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list documents")
		return
	}
	if docs == nil {
		docs = []CodeDocument{}
	}
	writeJSON(w, http.StatusOK, docs)
}

func (app *App) handleCreateDocument(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	workspaceID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	if ok, _ := app.IsWorkspaceMember(workspaceID, user.ID); !ok {
		writeError(w, http.StatusForbidden, "not a member of this workspace")
		return
	}

	var input struct {
		Title    string `json:"title"`
		Filename string `json:"filename"`
		Language string `json:"language"`
		Content  string `json:"content"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Input validation
	if input.Filename == "" {
		writeError(w, http.StatusBadRequest, "filename is required")
		return
	}
	if len(input.Content) > maxDocumentContent {
		writeError(w, http.StatusBadRequest, "content too large (max 1MB)")
		return
	}

	// Auto-detect language from filename if not provided
	if input.Language == "" {
		input.Language = detectLanguage(input.Filename)
	}

	// Set default title from filename if not provided
	if input.Title == "" {
		input.Title = strings.TrimSuffix(input.Filename, filepath.Ext(input.Filename))
		input.Title = strings.ReplaceAll(input.Title, "_", " ")
		input.Title = strings.ReplaceAll(input.Title, "-", " ")
		if input.Title == "" {
			input.Title = "Untitled"
		}
	}

	doc := &CodeDocument{}
	err = app.DB.QueryRow(`
		INSERT INTO code_documents (workspace_id, title, filename, language, content, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, workspace_id, title, filename, language, content, version, created_by, last_edited_by, created_at, updated_at
	`, workspaceID, input.Title, input.Filename, input.Language, input.Content, user.ID).Scan(
		&doc.ID, &doc.WorkspaceID, &doc.Title, &doc.Filename, &doc.Language,
		&doc.Content, &doc.Version, &doc.CreatedBy, &doc.LastEditedBy, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		log.Printf("[documents] create error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create document")
		return
	}

	doc.CreatedByName = user.Username
	writeJSON(w, http.StatusCreated, doc)
}

func (app *App) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid document id")
		return
	}

	q := `SELECT d.id, d.workspace_id, d.title, d.filename, d.language, d.content, d.version,
	             d.created_by, d.last_edited_by, d.created_at, d.updated_at,
	             cu.username, COALESCE(leu.username, '')
	      FROM code_documents d
	      JOIN users cu ON cu.id = d.created_by
	      LEFT JOIN users leu ON leu.id = d.last_edited_by
	      WHERE d.id = $1`

	docs, err := app.scanDocuments(q, id)
	if err != nil || len(docs) == 0 {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	// Authorization: check workspace membership
	if ok, _ := app.IsWorkspaceMember(docs[0].WorkspaceID, user.ID); !ok {
		writeError(w, http.StatusForbidden, "not a member of this workspace")
		return
	}

	writeJSON(w, http.StatusOK, docs[0])
}

func (app *App) handleUpdateDocument(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid document id")
		return
	}

	var input struct {
		Title    *string `json:"title"`
		Content  *string `json:"content"`
		Language *string `json:"language"`
		Version  int     `json:"version"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate content length
	if input.Content != nil && len(*input.Content) > maxDocumentContent {
		writeError(w, http.StatusBadRequest, "content too large (max 1MB)")
		return
	}

	// Check if document exists, get current version + workspace for auth
	var currentVersion int
	var wsID int64
	err = app.DB.QueryRow("SELECT version, workspace_id FROM code_documents WHERE id = $1", id).Scan(&currentVersion, &wsID)
	if err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	// Authorization: check workspace membership
	if ok, _ := app.IsWorkspaceMember(wsID, user.ID); !ok {
		writeError(w, http.StatusForbidden, "not a member of this workspace")
		return
	}

	// Optimistic locking check
	if input.Version != currentVersion {
		writeError(w, http.StatusConflict, "document was modified by another user")
		return
	}

	now := time.Now()
	result, err := app.DB.Exec(`
		UPDATE code_documents
		SET title = COALESCE($1, title),
		    content = COALESCE($2, content),
		    language = COALESCE($3, language),
		    version = version + 1,
		    last_edited_by = $4,
		    updated_at = $5
		WHERE id = $6 AND version = $7
	`, input.Title, input.Content, input.Language, user.ID, now, id, currentVersion)

	if err != nil {
		log.Printf("[documents] update error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update document")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusConflict, "document was modified by another user")
		return
	}

	// Fetch and return updated document
	q := `SELECT d.id, d.workspace_id, d.title, d.filename, d.language, d.content, d.version,
	             d.created_by, d.last_edited_by, d.created_at, d.updated_at,
	             cu.username, COALESCE(leu.username, '')
	      FROM code_documents d
	      JOIN users cu ON cu.id = d.created_by
	      LEFT JOIN users leu ON leu.id = d.last_edited_by
	      WHERE d.id = $1`

	docs, err := app.scanDocuments(q, id)
	if err != nil || len(docs) == 0 {
		writeError(w, http.StatusInternalServerError, "failed to fetch updated document")
		return
	}

	// Broadcast document edit event via WebSocket
	app.Hub.BroadcastDocumentEdit(id, user.ID, user.Username)

	writeJSON(w, http.StatusOK, docs[0])
}

func (app *App) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid document id")
		return
	}

	// Get document info for authorization
	var createdBy, wsID int64
	err = app.DB.QueryRow("SELECT created_by, workspace_id FROM code_documents WHERE id = $1", id).Scan(&createdBy, &wsID)
	if err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	// Authorization: must be workspace member AND document owner
	if ok, _ := app.IsWorkspaceMember(wsID, user.ID); !ok {
		writeError(w, http.StatusForbidden, "not a member of this workspace")
		return
	}
	if createdBy != user.ID {
		writeError(w, http.StatusForbidden, "only document owner can delete")
		return
	}

	_, err = app.DB.Exec(`DELETE FROM code_documents WHERE id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete document")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (app *App) handleExecuteDocument(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid document id")
		return
	}

	// Get document with workspace_id for authorization
	var language, content string
	var wsID int64
	err = app.DB.QueryRow("SELECT language, content, workspace_id FROM code_documents WHERE id = $1", id).Scan(&language, &content, &wsID)
	if err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	// Authorization: check workspace membership
	if ok, _ := app.IsWorkspaceMember(wsID, user.ID); !ok {
		writeError(w, http.StatusForbidden, "not a member of this workspace")
		return
	}

	// Execute in sandbox
	ctx := r.Context()
	result, err := app.Sandbox.Execute(ctx, ai.CodeExecutionRequest{
		Language: language,
		Code:     content,
	})

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"exit_code": -1,
			"stdout":    "",
			"stderr":    err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"exit_code": result.ExitCode,
		"stdout":    result.Stdout,
		"stderr":    result.Stderr,
		"duration":  result.Duration,
	})
}

// --- Helper Functions ---

func (app *App) scanDocuments(query string, args ...any) ([]CodeDocument, error) {
	rows, err := app.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []CodeDocument{}
	for rows.Next() {
		var d CodeDocument
		if err := rows.Scan(&d.ID, &d.WorkspaceID, &d.Title, &d.Filename, &d.Language,
			&d.Content, &d.Version, &d.CreatedBy, &d.LastEditedBy,
			&d.CreatedAt, &d.UpdatedAt, &d.CreatedByName, &d.LastEditedByName); err != nil {
			return nil, fmt.Errorf("scan document row: %w", err)
		}
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate document rows: %w", err)
	}
	return docs, nil
}

// detectLanguage determines the programming language from filename
func detectLanguage(filename string) string {
	// Check bare filenames first
	base := strings.ToLower(filepath.Base(filename))
	switch base {
	case "dockerfile":
		return "dockerfile"
	case "makefile", "gnumakefile":
		return "makefile"
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".py":
		return "python"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".go":
		return "go"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".rb":
		return "ruby"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".sh", ".bash":
		return "bash"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".scss", ".sass":
		return "scss"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	case ".sql":
		return "sql"
	default:
		return "text"
	}
}
