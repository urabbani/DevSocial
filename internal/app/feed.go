package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// --- Post Models ---

type Post struct {
	ID           int64          `json:"id"`
	WorkspaceID  int64          `json:"workspace_id"`
	AuthorID     *int64         `json:"author_id,omitempty"`
	ParentPostID *int64         `json:"parent_post_id,omitempty"`
	Content      string         `json:"content"`
	ContentHTML  string         `json:"content_html,omitempty"`
	IsAI         bool           `json:"is_ai"`
	PostType     string         `json:"post_type"`
	Pinned       bool           `json:"pinned"`
	CreatedAt    time.Time      `json:"created_at"`
	EditedAt     *time.Time     `json:"edited_at,omitempty"`
	Author       *User          `json:"author,omitempty"`
	Reactions    []PostReaction `json:"reactions,omitempty"`
	ReplyCount   int            `json:"reply_count"`
	LikeCount    int            `json:"like_count"`
}

type PostReaction struct {
	UserID   int64     `json:"user_id"`
	Username string    `json:"username"`
	Reaction string    `json:"reaction"`
	CreateAt time.Time `json:"created_at"`
}

type CreatePostInput struct {
	Content       string  `json:"content"`
	PostType      string  `json:"post_type"`
	ParentPostID  *int64  `json:"parent_post_id"`
	AttachmentIDs []int64 `json:"attachment_ids"`
}

// --- Feed Handlers ---

func (app *App) handleGetFeed(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	beforeID, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)

	var rows *sql.Rows
	if beforeID > 0 {
		rows, err = app.DB.Query(`
			SELECT p.id, p.workspace_id, p.author_id, p.parent_post_id, p.content, p.content_html,
			       p.is_ai, p.post_type, p.pinned, p.created_at, p.edited_at,
			       COALESCE(u.username, 'AI') as username, COALESCE(u.display_name, 'AI') as display_name,
			       COALESCE(u.avatar_url, '') as avatar_url,
			       (SELECT COUNT(*) FROM posts WHERE parent_post_id = p.id) as reply_count,
			       (SELECT COUNT(*) FROM post_reactions WHERE post_id = p.id AND reaction = 'like') as like_count
			FROM posts p
			LEFT JOIN users u ON u.id = p.author_id
			WHERE p.workspace_id = $1 AND p.parent_post_id IS NULL AND p.id < $2
			ORDER BY p.created_at DESC
			LIMIT $3
		`, workspaceID, beforeID, limit)
	} else {
		rows, err = app.DB.Query(`
			SELECT p.id, p.workspace_id, p.author_id, p.parent_post_id, p.content, p.content_html,
			       p.is_ai, p.post_type, p.pinned, p.created_at, p.edited_at,
			       COALESCE(u.username, 'AI') as username, COALESCE(u.display_name, 'AI') as display_name,
			       COALESCE(u.avatar_url, '') as avatar_url,
			       (SELECT COUNT(*) FROM posts WHERE parent_post_id = p.id) as reply_count,
			       (SELECT COUNT(*) FROM post_reactions WHERE post_id = p.id AND reaction = 'like') as like_count
			FROM posts p
			LEFT JOIN users u ON u.id = p.author_id
			WHERE p.workspace_id = $1 AND p.parent_post_id IS NULL
			ORDER BY p.created_at DESC
			LIMIT $2
		`, workspaceID, limit)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load feed")
		return
	}
	defer rows.Close()

	posts := []Post{}
	for rows.Next() {
		var p Post
		var authorID sql.NullInt64
		var parentID sql.NullInt64
		var contentHTML sql.NullString
		var editedAt sql.NullTime
		var username, displayName, avatarURL string

		if err := rows.Scan(&p.ID, &p.WorkspaceID, &authorID, &parentID, &p.Content, &contentHTML,
			&p.IsAI, &p.PostType, &p.Pinned, &p.CreatedAt, &editedAt,
			&username, &displayName, &avatarURL,
			&p.ReplyCount, &p.LikeCount); err != nil {
			continue
		}

		if authorID.Valid {
			p.AuthorID = &authorID.Int64
			p.Author = &User{ID: authorID.Int64, Username: username, DisplayName: displayName, AvatarURL: avatarURL}
		} else {
			p.Author = &User{Username: "AI", DisplayName: "AI Assistant"}
		}
		if parentID.Valid {
			p.ParentPostID = &parentID.Int64
		}
		if contentHTML.Valid {
			p.ContentHTML = contentHTML.String
		}
		if editedAt.Valid {
			p.EditedAt = &editedAt.Time
		}
		posts = append(posts, p)
	}

	if posts == nil {
		posts = []Post{}
	}
	writeJSON(w, http.StatusOK, posts)
}

func (app *App) handleCreatePost(w http.ResponseWriter, r *http.Request) {
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

	var input CreatePostInput
	if err := readJSON(r, &input); err != nil || input.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	if input.PostType == "" {
		input.PostType = "discussion"
	}

	contentHTML, _ := RenderMarkdown(input.Content)

	var parentID *int64
	if input.ParentPostID != nil && *input.ParentPostID > 0 {
		parentID = input.ParentPostID
	}

	p := &Post{}
	err = app.DB.QueryRow(`
		INSERT INTO posts (workspace_id, author_id, parent_post_id, content, content_html, post_type)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, workspace_id, author_id, parent_post_id, content, content_html, is_ai, post_type, pinned, created_at
	`, workspaceID, user.ID, parentID, input.Content, contentHTML, input.PostType).Scan(
		&p.ID, &p.WorkspaceID, &p.AuthorID, &p.ParentPostID,
		&p.Content, &p.ContentHTML, &p.IsAI, &p.PostType, &p.Pinned, &p.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create post")
		return
	}

	p.Author = user
	p.ReplyCount = 0
	p.LikeCount = 0

	// Wire attachments
	if len(input.AttachmentIDs) > 0 {
		for _, fileID := range input.AttachmentIDs {
			app.DB.Exec(`INSERT INTO post_attachments (post_id, file_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, p.ID, fileID)
		}
	}

	// Broadcast via WebSocket
	data, _ := json.Marshal(map[string]any{
		"type":       "new_post",
		"post":       p,
		"workspace_id": workspaceID,
	})
	app.Hub.BroadcastToWorkspace(workspaceID, data)

	writeJSON(w, http.StatusCreated, p)
}

func (app *App) handleGetPost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post id")
		return
	}

	p, err := app.getPostByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (app *App) handleTogglePostReaction(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post id")
		return
	}

	var input struct {
		Reaction string `json:"reaction"`
	}
	if err := readJSON(r, &input); err != nil || input.Reaction == "" {
		input.Reaction = "like"
	}

	// Toggle: delete if exists, insert if not
	var deleted bool
	err = app.DB.QueryRow(`
		DELETE FROM post_reactions WHERE post_id = $1 AND user_id = $2 AND reaction = $3
		RETURNING true
	`, id, user.ID, input.Reaction).Scan(&deleted)

	if err != nil {
		app.DB.Exec(`
			INSERT INTO post_reactions (post_id, user_id, reaction) VALUES ($1, $2, $3)
		`, id, user.ID, input.Reaction)
	}

	// Return updated like count
	var count int
	app.DB.QueryRow(`SELECT COUNT(*) FROM post_reactions WHERE post_id = $1 AND reaction = 'like'`, id).Scan(&count)
	writeJSON(w, http.StatusOK, map[string]int{"like_count": count})
}

func (app *App) handleGetPostReplies(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post id")
		return
	}

	rows, err := app.DB.Query(`
		SELECT p.id, p.workspace_id, p.author_id, p.content, p.content_html, p.is_ai, p.created_at,
		       COALESCE(u.username, 'AI'), COALESCE(u.display_name, 'AI'), COALESCE(u.avatar_url, '')
		FROM posts p
		LEFT JOIN users u ON u.id = p.author_id
		WHERE p.parent_post_id = $1
		ORDER BY p.created_at ASC
	`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load replies")
		return
	}
	defer rows.Close()

	replies := []Post{}
	for rows.Next() {
		var p Post
		var authorID sql.NullInt64
		var contentHTML sql.NullString
		var username, displayName, avatarURL string
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &authorID, &p.Content, &contentHTML,
			&p.IsAI, &p.CreatedAt, &username, &displayName, &avatarURL); err != nil {
			continue
		}
		if authorID.Valid {
			p.AuthorID = &authorID.Int64
			p.Author = &User{ID: authorID.Int64, Username: username, DisplayName: displayName, AvatarURL: avatarURL}
		} else {
			p.Author = &User{Username: "AI", DisplayName: "AI Assistant"}
		}
		if contentHTML.Valid {
			p.ContentHTML = contentHTML.String
		}
		p.ParentPostID = &id
		replies = append(replies, p)
	}
	if replies == nil {
		replies = []Post{}
	}
	writeJSON(w, http.StatusOK, replies)
}

func (app *App) handleDeletePost(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid post id")
		return
	}

	res, err := app.DB.Exec(`DELETE FROM posts WHERE id = $1 AND author_id = $2`, id, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete post")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusForbidden, "not your post")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (app *App) getPostByID(id int64) (*Post, error) {
	p := &Post{}
	var authorID sql.NullInt64
	var parentID sql.NullInt64
	var contentHTML sql.NullString
	var editedAt sql.NullTime
	var username, displayName, avatarURL string

	err := app.DB.QueryRow(`
		SELECT p.id, p.workspace_id, p.author_id, p.parent_post_id, p.content, p.content_html,
		       p.is_ai, p.post_type, p.pinned, p.created_at, p.edited_at,
		       COALESCE(u.username, 'AI'), COALESCE(u.display_name, 'AI'), COALESCE(u.avatar_url, ''),
		       (SELECT COUNT(*) FROM posts WHERE parent_post_id = p.id),
		       (SELECT COUNT(*) FROM post_reactions WHERE post_id = p.id AND reaction = 'like')
		FROM posts p
		LEFT JOIN users u ON u.id = p.author_id
		WHERE p.id = $1
	`, id).Scan(&p.ID, &p.WorkspaceID, &authorID, &parentID, &p.Content, &contentHTML,
		&p.IsAI, &p.PostType, &p.Pinned, &p.CreatedAt, &editedAt,
		&username, &displayName, &avatarURL, &p.ReplyCount, &p.LikeCount)
	if err != nil {
		return nil, err
	}

	if authorID.Valid {
		p.AuthorID = &authorID.Int64
		p.Author = &User{ID: authorID.Int64, Username: username, DisplayName: displayName, AvatarURL: avatarURL}
	} else {
		p.Author = &User{Username: "AI", DisplayName: "AI Assistant"}
	}
	if parentID.Valid {
		p.ParentPostID = &parentID.Int64
	}
	if contentHTML.Valid {
		p.ContentHTML = contentHTML.String
	}
	if editedAt.Valid {
		p.EditedAt = &editedAt.Time
	}
	return p, nil
}

// AIPost creates a post on behalf of the AI assistant.
func (app *App) AIPost(workspaceID int64, content string, postType string) error {
	contentHTML, _ := RenderMarkdown(content)
	_, err := app.DB.Exec(`
		INSERT INTO posts (workspace_id, author_id, content, content_html, is_ai, post_type)
		VALUES ($1, NULL, $2, $3, true, $4)
	`, workspaceID, content, contentHTML, postType)

	if err == nil {
		data, _ := json.Marshal(map[string]any{
			"type":         "new_post",
			"workspace_id": workspaceID,
		})
		app.Hub.BroadcastToWorkspace(workspaceID, data)
	}
	return err
}

