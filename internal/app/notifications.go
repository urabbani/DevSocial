package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
)

// Notification represents a user notification.
type Notification struct {
	ID              int64             `json:"id"`
	UserID          int64             `json:"user_id"`
	Type            string            `json:"type"`            // mention, reaction, task_assigned, post_reply
	SourceUserID    *int64            `json:"source_user_id"`
	SourceUser      *User             `json:"source_user,omitempty"`
	SourceMessageID *int64            `json:"source_message_id,omitempty"`
	SourcePostID    *int64            `json:"source_post_id,omitempty"`
	SourceTaskID    *int64            `json:"source_task_id,omitempty"`
	Read            bool              `json:"read"`
	Data            map[string]any    `json:"data,omitempty"`
	CreatedAt       string            `json:"created_at"`
}

// mentionRegex finds @username mentions in text
var mentionRegex = regexp.MustCompile(`@(\w{1,30})`)

// createNotification creates a new notification if one doesn't already exist.
func (app *App) createNotification(userID int64, notificationType string, sourceUserID int64, data map[string]any, sourceMessageID, sourcePostID, sourceTaskID int64) error {
	// Check for duplicate
	var existingID int64
	err := app.DB.QueryRow(`
		SELECT id FROM notifications
		WHERE user_id = $1 AND type = $2 AND source_user_id = $3
			AND COALESCE(source_message_id, 0) = COALESCE($4, 0)
			AND COALESCE(source_post_id, 0) = COALESCE($5, 0)
			AND COALESCE(source_task_id, 0) = COALESCE($6, 0)
		LIMIT 1
	`, userID, notificationType, sourceUserID, sourceMessageID, sourcePostID, sourceTaskID).Scan(&existingID)

	if err == nil {
		// Notification already exists, skip
		return nil
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("check duplicate notification: %w", err)
	}

	// Don't notify yourself
	if userID == sourceUserID {
		return nil
	}

	// Serialize data
	dataJSON, _ := json.Marshal(data)

	// Insert notification
	var id int64
	err = app.DB.QueryRow(`
		INSERT INTO notifications (user_id, type, source_user_id, source_message_id, source_post_id, source_task_id, data)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, userID, notificationType, sourceUserID, nullInt(sourceMessageID), nullInt(sourcePostID), nullInt(sourceTaskID), dataJSON).Scan(&id)

	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	// Fetch complete notification for WebSocket
	notif, err := app.getNotificationByID(id)
	if err != nil {
		return err
	}

	// Broadcast via WebSocket
	app.broadcastNotification(notif)
	log.Printf("[notifications] created %s notification for user %d", notificationType, userID)

	return nil
}

// getNotificationByID fetches a notification with user details.
func (app *App) getNotificationByID(id int64) (*Notification, error) {
	var notif Notification
	var dataJSON []byte
	var sourceUserID, sourceMessageID, sourcePostID, sourceTaskID sql.NullInt64

	err := app.DB.QueryRow(`
		SELECT n.id, n.user_id, n.type, n.source_user_id, n.source_message_id, n.source_post_id, n.source_task_id, n.read, n.data, n.created_at,
			u.id, u.username, u.display_name, u.avatar_url
		FROM notifications n
		LEFT JOIN users u ON n.source_user_id = u.id
		WHERE n.id = $1
	`, id).Scan(
		&notif.ID, &notif.UserID, &notif.Type, &sourceUserID, &sourceMessageID, &sourcePostID, &sourceTaskID,
		&notif.Read, &dataJSON, &notif.CreatedAt,
		&notif.SourceUser.ID, &notif.SourceUser.Username, &notif.SourceUser.DisplayName, &notif.SourceUser.AvatarURL,
	)

	if err != nil {
		return nil, err
	}

	if sourceUserID.Valid {
		uid := sourceUserID.Int64
		notif.SourceUserID = &uid
	}
	if sourceMessageID.Valid {
		id := sourceMessageID.Int64
		notif.SourceMessageID = &id
	}
	if sourcePostID.Valid {
		id := sourcePostID.Int64
		notif.SourcePostID = &id
	}
	if sourceTaskID.Valid {
		id := sourceTaskID.Int64
		notif.SourceTaskID = &id
	}

	if len(dataJSON) > 0 {
		json.Unmarshal(dataJSON, &notif.Data)
	}

	return &notif, nil
}

// extractMentions finds @mentions in text and returns usernames.
func extractMentions(text string) []string {
	matches := mentionRegex.FindAllStringSubmatch(text, -1)
	usernames := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) > 1 {
			username := m[1]
			if !seen[username] {
				usernames = append(usernames, username)
				seen[username] = true
			}
		}
	}
	return usernames
}

// notifyMentions creates notifications for @mentioned users.
func (app *App) notifyMentions(content string, authorID int64, channelID int64, messageID int64) error {
	usernames := extractMentions(content)
	if len(usernames) == 0 {
		return nil
	}

	// Get workspace ID
	var workspaceID int64
	err := app.DB.QueryRow("SELECT workspace_id FROM channels WHERE id = $1", channelID).Scan(&workspaceID)
	if err != nil {
		return err
	}

	// Find mentioned users in workspace
	for _, username := range usernames {
		var userID int64
		err := app.DB.QueryRow(`
			SELECT user_id FROM workspace_members
			WHERE workspace_id = $1 AND user_id IN (SELECT id FROM users WHERE username = $2)
		`, workspaceID, username).Scan(&userID)

		if err == nil {
			// Create mention notification
			data := map[string]any{
				"mention_text": content[:100], // First 100 chars
				"channel_id":  channelID,
			}
			app.createNotification(userID, "mention", authorID, data, messageID, 0, 0)
		}
	}

	return nil
}

// notifyPostReaction creates a notification when someone reacts to a post.
func (app *App) notifyPostReaction(postID, reactorID int64) error {
	// Get post author
	var authorID int64
	var content string
	err := app.DB.QueryRow("SELECT author_id, content FROM posts WHERE id = $1", postID).Scan(&authorID, &content)
	if err != nil {
		return err
	}

	data := map[string]any{
		"post_preview": content[:100],
	}
	return app.createNotification(authorID, "reaction", reactorID, data, 0, postID, 0)
}

// notifyTaskAssignment creates a notification when a task is assigned.
func (app *App) notifyTaskAssignment(taskID, assigneeID int64, creatorID int64) error {
	// Get task details
	var title string
	err := app.DB.QueryRow("SELECT title FROM tasks WHERE id = $1", taskID).Scan(&title)
	if err != nil {
		return err
	}

	data := map[string]any{
		"task_title": title,
	}
	return app.createNotification(assigneeID, "task_assigned", creatorID, data, 0, 0, taskID)
}

// notifyPostReply creates a notification when someone replies to a post.
func (app *App) notifyPostReply(postID, replierID int64) error {
	// Get parent post author
	var parentAuthorID int64
	var content string
	err := app.DB.QueryRow(`
		SELECT p.author_id, p.content FROM posts p
		JOIN posts r ON r.parent_post_id = p.id
		WHERE r.id = $1
	`, postID).Scan(&parentAuthorID, &content)

	if err != nil {
		return err
	}

	data := map[string]any{
		"post_preview": content[:100],
	}
	return app.createNotification(parentAuthorID, "post_reply", replierID, data, 0, postID, 0)
}

// broadcastNotification sends a notification via WebSocket.
func (app *App) broadcastNotification(notif *Notification) {
	data, err := json.Marshal(WSMessageOut{
		Type:         "notification",
		Notification: notif,
	})
	if err != nil {
		return
	}
	// Send to user's specific WebSocket connection
	app.Hub.SendToUser(notif.UserID, data)
}

// nullInt converts an int64 to sql.NullInt64.
func nullInt(n int64) sql.NullInt64 {
	if n == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: n, Valid: true}
}
