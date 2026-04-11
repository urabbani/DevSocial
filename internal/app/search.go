package app

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"devsocial/internal/search"
)

// --- Search ---

type SearchResult struct {
	Type    string  `json:"type"`    // "message", "post", "task", "file"
	ID      int64   `json:"id"`
	Title   string  `json:"title"`
	Preview string  `json:"preview"`
	Author  string  `json:"author,omitempty"`
	Date    string  `json:"date"`
	Score   float64 `json:"score,omitempty"` // Relevance score for semantic search
}

func (app *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []SearchResult{})
		return
	}

	workspaceID, _ := strconv.ParseInt(r.URL.Query().Get("workspace_id"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	// Check search mode: keyword, semantic, or hybrid
	searchMode := r.URL.Query().Get("mode")
	if searchMode == "" {
		searchMode = "keyword" // default
	}

	var results []SearchResult

	ctx := r.Context()

	switch searchMode {
	case "semantic":
		if app.SemanticSearcher != nil {
			semanticResults, e := app.SemanticSearcher.SemanticSearch(ctx, q, workspaceID, limit)
			if e != nil {
				writeError(w, http.StatusInternalServerError, "semantic search failed")
				return
			}
			results = convertSemanticResults(semanticResults)
		} else {
			// Fallback to keyword search
			results = app.keywordSearch(q, workspaceID, limit)
		}
	case "hybrid":
		if app.SemanticSearcher != nil {
			semanticResults, e := app.SemanticSearcher.HybridSearch(ctx, q, workspaceID, limit)
			if e != nil {
				writeError(w, http.StatusInternalServerError, "hybrid search failed")
				return
			}
			results = convertSemanticResults(semanticResults)
		} else {
			// Fallback to keyword search
			results = app.keywordSearch(q, workspaceID, limit)
		}
	default:
		results = app.keywordSearch(q, workspaceID, limit)
	}

	if results == nil {
		results = []SearchResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

// keywordSearch performs the original keyword-based search.
func (app *App) keywordSearch(q string, workspaceID int64, limit int) []SearchResult {
	results := []SearchResult{}
	pattern := "%" + strings.ToLower(q) + "%"

	// Search messages
	if workspaceID > 0 {
		rows, err := app.DB.Query(`
			SELECT m.id, m.content, COALESCE(u.username, 'AI'), m.created_at::text
			FROM messages m
			LEFT JOIN users u ON u.id = m.author_id
			JOIN channels c ON c.id = m.channel_id
			WHERE c.workspace_id = $1 AND LOWER(m.content) LIKE $2
			ORDER BY m.created_at DESC LIMIT $3
		`, workspaceID, pattern, limit)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var content, author, date string
				if rows.Scan(&id, &content, &author, &date) == nil {
					preview := content
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					results = append(results, SearchResult{Type: "message", ID: id, Title: author, Preview: preview, Author: author, Date: date})
				}
			}
		}
	}

	// Search posts
	if workspaceID > 0 {
		rows, err := app.DB.Query(`
			SELECT p.id, p.content, COALESCE(u.username, 'AI'), p.created_at::text
			FROM posts p
			LEFT JOIN users u ON u.id = p.author_id
			WHERE p.workspace_id = $1 AND LOWER(p.content) LIKE $2
			ORDER BY p.created_at DESC LIMIT $3
		`, workspaceID, pattern, limit)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var content, author, date string
				if rows.Scan(&id, &content, &author, &date) == nil {
					preview := content
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					title := preview
					if len(title) > 80 {
						title = title[:80]
					}
					results = append(results, SearchResult{Type: "post", ID: id, Title: title, Preview: preview, Author: author, Date: date})
				}
			}
		}
	}

	// Search tasks
	if workspaceID > 0 {
		rows, err := app.DB.Query(`
			SELECT t.id, t.title, t.status, cu.username, t.created_at::text
			FROM tasks t
			JOIN users cu ON cu.id = t.creator_id
			WHERE t.workspace_id = $1 AND (LOWER(t.title) LIKE $2 OR LOWER(t.description) LIKE $2)
			ORDER BY t.created_at DESC LIMIT $3
		`, workspaceID, pattern, limit)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var title, status, creator, date string
				if rows.Scan(&id, &title, &status, &creator, &date) == nil {
					results = append(results, SearchResult{Type: "task", ID: id, Title: title, Preview: status, Author: creator, Date: date})
				}
			}
		}
	}

	// Search files
	if workspaceID > 0 {
		rows, err := app.DB.Query(`
			SELECT f.id, f.filename, u.username, f.created_at::text
			FROM files f
			JOIN users u ON u.id = f.uploader_id
			WHERE f.workspace_id = $1 AND LOWER(f.filename) LIKE $2
			ORDER BY f.created_at DESC LIMIT $3
		`, workspaceID, pattern, limit)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var filename, uploader, date string
				if rows.Scan(&id, &filename, &uploader, &date) == nil {
					results = append(results, SearchResult{Type: "file", ID: id, Title: filename, Preview: "uploaded by " + uploader, Author: uploader, Date: date})
				}
			}
		}
	}

	return results
}

// convertSemanticResults converts search.SemanticResult to app.SearchResult.
func convertSemanticResults(semanticResults []search.SearchResult) []SearchResult {
	results := make([]SearchResult, len(semanticResults))
	for i, sr := range semanticResults {
		results[i] = SearchResult{
			Type:    sr.Type,
			ID:      sr.ID,
			Title:   sr.Title,
			Preview: sr.Preview,
			Author:  sr.Author,
			Date:    sr.Date,
			Score:   sr.Score,
		}
	}
	return results
}

// handleReindexEmbeddings triggers a full re-indexing of embeddings.
func (app *App) handleReindexEmbeddings(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if !app.isAdmin(user) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	if app.Embedder == nil {
		writeError(w, http.StatusServiceUnavailable, "embedder not available")
		return
	}

	// Run re-indexing in background
	go func() {
		ctx := context.Background()
		if err := app.Embedder.MigrateAll(ctx); err != nil {
			// Log error but don't block response
			return
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "reindexing started"})
}
