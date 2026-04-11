package app

import (
	"net/http"
	"strconv"
	"strings"
)

// --- Search ---

type SearchResult struct {
	Type    string `json:"type"`    // "message", "post", "task", "file"
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Preview string `json:"preview"`
	Author  string `json:"author,omitempty"`
	Date    string `json:"date"`
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

	if results == nil {
		results = []SearchResult{}
	}
	writeJSON(w, http.StatusOK, results)
}
