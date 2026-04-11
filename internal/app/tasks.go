package app

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"
)

// --- Task Models ---

type Task struct {
	ID          int64      `json:"id"`
	WorkspaceID int64      `json:"workspace_id"`
	ChannelID   *int64     `json:"channel_id,omitempty"`
	CreatorID   int64      `json:"creator_id"`
	AssigneeID  *int64     `json:"assignee_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	// Joined
	CreatorName  string `json:"creator_name,omitempty"`
	AssigneeName string `json:"assignee_name,omitempty"`
}

// --- Task Handlers ---

func (app *App) handleListTasks(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	status := r.URL.Query().Get("status")

	var rows []Task
	if status != "" {
		q := `SELECT t.id, t.workspace_id, t.channel_id, t.creator_id, t.assignee_id,
		             t.title, t.description, t.status, t.priority, t.due_date, t.created_at, t.updated_at,
		             cu.username, COALESCE(au.username, '')
			      FROM tasks t
			      JOIN users cu ON cu.id = t.creator_id
			      LEFT JOIN users au ON au.id = t.assignee_id
			      WHERE t.workspace_id = $1 AND t.status = $2
			      ORDER BY CASE t.priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END, t.created_at DESC`
		rows, err = app.scanTasks(q, workspaceID, status)
	} else {
		q := `SELECT t.id, t.workspace_id, t.channel_id, t.creator_id, t.assignee_id,
		             t.title, t.description, t.status, t.priority, t.due_date, t.created_at, t.updated_at,
		             cu.username, COALESCE(au.username, '')
			      FROM tasks t
			      JOIN users cu ON cu.id = t.creator_id
			      LEFT JOIN users au ON au.id = t.assignee_id
			      WHERE t.workspace_id = $1
			      ORDER BY CASE t.priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END, t.created_at DESC`
		rows, err = app.scanTasks(q, workspaceID)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	if rows == nil {
		rows = []Task{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (app *App) handleCreateTask(w http.ResponseWriter, r *http.Request) {
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
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
		AssigneeID  *int64 `json:"assignee_id"`
		ChannelID   *int64 `json:"channel_id"`
	}
	if err := readJSON(r, &input); err != nil || input.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if input.Priority == "" || !isValidPriority(input.Priority) {
		input.Priority = "medium"
	}

	t := &Task{}
	err = app.DB.QueryRow(`
		INSERT INTO tasks (workspace_id, channel_id, creator_id, assignee_id, title, description, priority)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, workspace_id, channel_id, creator_id, assignee_id, title, description, status, priority, due_date, created_at, updated_at
	`, workspaceID, input.ChannelID, user.ID, input.AssigneeID, input.Title, input.Description, input.Priority).Scan(
		&t.ID, &t.WorkspaceID, &t.ChannelID, &t.CreatorID, &t.AssigneeID,
		&t.Title, &t.Description, &t.Status, &t.Priority, &t.DueDate, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	// Notify assignee if different from creator
	if t.AssigneeID != nil && *t.AssigneeID != user.ID {
		go app.notifyTaskAssignment(t.ID, *t.AssigneeID, user.ID)
	}
	t.CreatorName = user.Username
	writeJSON(w, http.StatusCreated, t)
}

func (app *App) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}

	var input struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
		Priority    *string `json:"priority"`
		AssigneeID  *int64  `json:"assignee_id"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Status != nil && !isValidStatus(*input.Status) {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}
	if input.Priority != nil && !isValidPriority(*input.Priority) {
		writeError(w, http.StatusBadRequest, "invalid priority")
		return
	}

	now := time.Now()
	if input.Title != nil {
		if _, err := app.DB.Exec(`UPDATE tasks SET title = $1, updated_at = $2 WHERE id = $3`, *input.Title, now, id); err != nil {
			log.Printf("[tasks] update error: %v", err)
		}
	}
	if input.Description != nil {
		if _, err := app.DB.Exec(`UPDATE tasks SET description = $1, updated_at = $2 WHERE id = $3`, *input.Description, now, id); err != nil {
			log.Printf("[tasks] update error: %v", err)
		}
	}
	if input.Status != nil {
		if _, err := app.DB.Exec(`UPDATE tasks SET status = $1, updated_at = $2 WHERE id = $3`, *input.Status, now, id); err != nil {
			log.Printf("[tasks] update error: %v", err)
		}
	}
	if input.Priority != nil {
		if _, err := app.DB.Exec(`UPDATE tasks SET priority = $1, updated_at = $2 WHERE id = $3`, *input.Priority, now, id); err != nil {
			log.Printf("[tasks] update error: %v", err)
		}
	}
	if input.AssigneeID != nil {
		// Get current assignee to check if it changed
		var oldAssigneeID sql.NullInt64
		app.DB.QueryRow("SELECT assignee_id FROM tasks WHERE id = $1", id).Scan(&oldAssigneeID)

		if _, err := app.DB.Exec(`UPDATE tasks SET assignee_id = $1, updated_at = $2 WHERE id = $3`, *input.AssigneeID, now, id); err != nil {
			log.Printf("[tasks] update error: %v", err)
		}

		// Notify new assignee if different from old assignee and not self
		if *input.AssigneeID != 0 && (!oldAssigneeID.Valid || oldAssigneeID.Int64 != *input.AssigneeID) {
			if *input.AssigneeID != user.ID {
				go app.notifyTaskAssignment(id, *input.AssigneeID, user.ID)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (app *App) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}
	app.DB.Exec(`DELETE FROM tasks WHERE id = $1`, id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (app *App) scanTasks(query string, args ...any) ([]Task, error) {
	rows, err := app.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.ChannelID, &t.CreatorID, &t.AssigneeID,
			&t.Title, &t.Description, &t.Status, &t.Priority, &t.DueDate,
			&t.CreatedAt, &t.UpdatedAt, &t.CreatorName, &t.AssigneeName); err != nil {
			log.Printf("[tasks] scan error: %v", err)
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func isValidStatus(s string) bool {
	switch s {
	case "todo", "in_progress", "review", "done":
		return true
	}
	return false
}

func isValidPriority(p string) bool {
	switch p {
	case "low", "medium", "high", "urgent":
		return true
	}
	return false
}
