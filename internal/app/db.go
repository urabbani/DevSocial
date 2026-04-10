package app

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// InitDB connects to PostgreSQL and runs migrations.
func InitDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	return db, nil
}

func runMigrations(db *sql.DB) error {
	migrationsDir := projectPath("internal/database/migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := filepath.Join(migrationsDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			// PostgreSQL handles idempotency via IF NOT EXISTS / CREATE EXTENSION IF NOT EXISTS
			// For tables, we check if they already exist
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("exec migration %s: %w", entry.Name(), err)
			}
		}
	}
	return nil
}

// --- Users ---

func (app *App) UpsertUser(githubID int64, username, displayName, avatarURL string) (*User, error) {
	u := &User{}
	err := app.DB.QueryRow(`
		INSERT INTO users (github_id, username, display_name, avatar_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (github_id) DO UPDATE SET
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url
		RETURNING id, github_id, username, display_name, avatar_url, bio, created_at
	`, githubID, username, displayName, avatarURL).Scan(
		&u.ID, &u.GitHubID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Bio, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (app *App) GetUserByGitHubID(githubID int64) (*User, error) {
	u := &User{}
	err := app.DB.QueryRow(`
		SELECT id, github_id, username, display_name, avatar_url, bio, created_at
		FROM users WHERE github_id = $1
	`, githubID).Scan(&u.ID, &u.GitHubID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Bio, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (app *App) GetUserByID(id int64) (*User, error) {
	u := &User{}
	err := app.DB.QueryRow(`
		SELECT id, github_id, username, display_name, avatar_url, bio, created_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.GitHubID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Bio, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (app *App) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := app.DB.QueryRow(`
		SELECT id, github_id, username, display_name, avatar_url, bio, created_at
		FROM users WHERE username = $1
	`, username).Scan(&u.ID, &u.GitHubID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Bio, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// --- Sessions ---

func (app *App) CreateSession(userID int64) (string, error) {
	var token string
	err := app.DB.QueryRow(`
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES (encode(gen_random_bytes(32), 'hex'), $1, $2)
		RETURNING token
	`, userID, time.Now().Add(30*24*time.Hour)).Scan(&token)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (app *App) GetUserBySession(token string) (*User, error) {
	u := &User{}
	err := app.DB.QueryRow(`
		SELECT u.id, u.github_id, u.username, u.display_name, u.avatar_url, u.bio, u.created_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = $1 AND s.expires_at > $2
	`, token, time.Now()).Scan(&u.ID, &u.GitHubID, &u.Username, &u.DisplayName, &u.AvatarURL, &u.Bio, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (app *App) DeleteSession(token string) error {
	_, err := app.DB.Exec(`DELETE FROM sessions WHERE token = $1`, token)
	return err
}

func (app *App) CleanExpiredSessions() error {
	_, err := app.DB.Exec(`DELETE FROM sessions WHERE expires_at < $1`, time.Now())
	return err
}

// --- Workspaces ---

func (app *App) CreateWorkspace(name, description, slug string, ownerID int64) (*Workspace, error) {
	ws := &Workspace{}
	err := app.DB.QueryRow(`
		INSERT INTO workspaces (name, description, slug)
		VALUES ($1, $2, $3)
		RETURNING id, name, description, slug, created_at
	`, name, description, slug).Scan(&ws.ID, &ws.Name, &ws.Description, &ws.Slug, &ws.CreatedAt)
	if err != nil {
		return nil, err
	}

	// Add owner as member
	if _, err := app.DB.Exec(`
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, ws.ID, ownerID); err != nil {
		return nil, err
	}

	// Create default channels
	if _, err := app.DB.Exec(`
		INSERT INTO channels (workspace_id, name, description, type, position)
		VALUES ($1, 'general', 'General discussion', 'text', 0),
		       ($1, 'ai', 'AI assistant', 'ai', 1)
	`, ws.ID); err != nil {
		return nil, err
	}

	return ws, nil
}

func (app *App) GetWorkspace(id int64) (*Workspace, error) {
	ws := &Workspace{}
	err := app.DB.QueryRow(`
		SELECT id, name, description, slug, created_at FROM workspaces WHERE id = $1
	`, id).Scan(&ws.ID, &ws.Name, &ws.Description, &ws.Slug, &ws.CreatedAt)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

func (app *App) GetWorkspaceBySlug(slug string) (*Workspace, error) {
	ws := &Workspace{}
	err := app.DB.QueryRow(`
		SELECT id, name, description, slug, created_at FROM workspaces WHERE slug = $1
	`, slug).Scan(&ws.ID, &ws.Name, &ws.Description, &ws.Slug, &ws.CreatedAt)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

func (app *App) ListUserWorkspaces(userID int64) ([]*Workspace, error) {
	rows, err := app.DB.Query(`
		SELECT w.id, w.name, w.description, w.slug, w.created_at,
		       (SELECT COUNT(*) FROM workspace_members WHERE workspace_id = w.id) as member_count
		FROM workspaces w
		JOIN workspace_members wm ON wm.workspace_id = w.id
		WHERE wm.user_id = $1
		ORDER BY w.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []*Workspace
	for rows.Next() {
		ws := &Workspace{IsMember: true}
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.Slug, &ws.CreatedAt, &ws.MemberCount); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, rows.Err()
}

func (app *App) UpdateWorkspace(id, userID int64, name, description string) error {
	// Check ownership
	var role string
	if err := app.DB.QueryRow(`
		SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2
	`, id, userID).Scan(&role); err != nil {
		return fmt.Errorf("not a member of workspace")
	}
	if role != "owner" && role != "admin" {
		return fmt.Errorf("insufficient permissions")
	}

	_, err := app.DB.Exec(`
		UPDATE workspaces SET name = $1, description = $2 WHERE id = $3
	`, name, description, id)
	return err
}

func (app *App) DeleteWorkspace(id, userID int64) error {
	var role string
	if err := app.DB.QueryRow(`
		SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2
	`, id, userID).Scan(&role); err != nil {
		return fmt.Errorf("not a member of workspace")
	}
	if role != "owner" {
		return fmt.Errorf("only owner can delete workspace")
	}

	_, err := app.DB.Exec(`DELETE FROM workspaces WHERE id = $1`, id)
	return err
}

// --- Workspace Members ---

func (app *App) AddWorkspaceMember(workspaceID, userID int64, role string) error {
	// Check inviter is admin or owner
	if role != "member" {
		return fmt.Errorf("can only add members; use UpdateMemberRole for admin/owner")
	}
	_, err := app.DB.Exec(`
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, workspaceID, userID, role)
	return err
}

func (app *App) RemoveWorkspaceMember(workspaceID, targetUserID int64) error {
	_, err := app.DB.Exec(`
		DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2
	`, workspaceID, targetUserID)
	return err
}

func (app *App) UpdateMemberRole(workspaceID, targetUserID int64, role string) error {
	_, err := app.DB.Exec(`
		UPDATE workspace_members SET role = $1 WHERE workspace_id = $2 AND user_id = $3
	`, role, workspaceID, targetUserID)
	return err
}

func (app *App) ListWorkspaceMembers(workspaceID int64) ([]*WorkspaceMember, error) {
	rows, err := app.DB.Query(`
		SELECT wm.user_id, u.username, wm.role, wm.joined_at
		FROM workspace_members wm
		JOIN users u ON u.id = wm.user_id
		WHERE wm.workspace_id = $1
		ORDER BY wm.joined_at
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*WorkspaceMember
	for rows.Next() {
		m := &WorkspaceMember{}
		if err := rows.Scan(&m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (app *App) IsWorkspaceMember(workspaceID, userID int64) (bool, error) {
	var count int
	err := app.DB.QueryRow(`
		SELECT COUNT(*) FROM workspace_members WHERE workspace_id = $1 AND user_id = $2
	`, workspaceID, userID).Scan(&count)
	return count > 0, err
}

func (app *App) GetWorkspaceMemberRole(workspaceID, userID int64) (string, error) {
	var role string
	err := app.DB.QueryRow(`
		SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2
	`, workspaceID, userID).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

// --- Channels ---

func (app *App) CreateChannel(workspaceID int64, name, description, channelType string, position int) (*Channel, error) {
	ch := &Channel{}
	err := app.DB.QueryRow(`
		INSERT INTO channels (workspace_id, name, description, type, position)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, workspace_id, name, description, type, position, created_at
	`, workspaceID, name, description, channelType, position).Scan(
		&ch.ID, &ch.WorkspaceID, &ch.Name, &ch.Description, &ch.Type, &ch.Position, &ch.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func (app *App) GetChannel(id int64) (*Channel, error) {
	ch := &Channel{}
	err := app.DB.QueryRow(`
		SELECT id, workspace_id, name, description, type, position, created_at
		FROM channels WHERE id = $1
	`, id).Scan(&ch.ID, &ch.WorkspaceID, &ch.Name, &ch.Description, &ch.Type, &ch.Position, &ch.CreatedAt)
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func (app *App) ListWorkspaceChannels(workspaceID int64, userID int64) ([]*Channel, error) {
	rows, err := app.DB.Query(`
		SELECT c.id, c.workspace_id, c.name, c.description, c.type, c.position, c.created_at,
		       COALESCE((SELECT COUNT(*) FROM messages m WHERE m.channel_id = c.id AND m.id > COALESCE(cu.last_read_message_id, 0)), 0) as unread_count
		FROM channels c
		LEFT JOIN channel_unreads cu ON cu.channel_id = c.id AND cu.user_id = $2
		WHERE c.workspace_id = $1
		ORDER BY c.position
	`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []*Channel
	for rows.Next() {
		ch := &Channel{}
		if err := rows.Scan(&ch.ID, &ch.WorkspaceID, &ch.Name, &ch.Description, &ch.Type, &ch.Position, &ch.CreatedAt, &ch.UnreadCount); err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

func (app *App) UpdateChannel(id int64, name, description string) error {
	_, err := app.DB.Exec(`
		UPDATE channels SET name = $1, description = $2 WHERE id = $3
	`, name, description, id)
	return err
}

func (app *App) DeleteChannel(id int64) error {
	_, err := app.DB.Exec(`DELETE FROM channels WHERE id = $1`, id)
	return err
}

// --- Messages ---

func (app *App) CreateMessage(channelID int64, authorID *int64, content, contentHTML string, isAI, isSystem bool) (*Message, error) {
	msg := &Message{}
	err := app.DB.QueryRow(`
		INSERT INTO messages (channel_id, author_id, content, content_html, is_ai, is_system)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, channel_id, author_id, content, content_html, is_ai, is_system, created_at
	`, channelID, authorID, content, contentHTML, isAI, isSystem).Scan(
		&msg.ID, &msg.ChannelID, &msg.AuthorID, &msg.Content, &msg.ContentHTML,
		&msg.IsAI, &msg.IsSystem, &msg.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (app *App) GetMessage(id int64) (*Message, error) {
	msg := &Message{}
	var authorID sql.NullInt64
	var editedAt sql.NullTime
	err := app.DB.QueryRow(`
		SELECT id, channel_id, author_id, content, content_html, is_ai, is_system, created_at, edited_at
		FROM messages WHERE id = $1
	`, id).Scan(&msg.ID, &msg.ChannelID, &authorID, &msg.Content, &msg.ContentHTML,
		&msg.IsAI, &msg.IsSystem, &msg.CreatedAt, &editedAt,
	)
	if err != nil {
		return nil, err
	}
	if authorID.Valid {
		msg.AuthorID = &authorID.Int64
	}
	if editedAt.Valid {
		msg.EditedAt = &editedAt.Time
	}
	return msg, nil
}

func (app *App) GetMessageWithAuthor(id int64) (*Message, error) {
	msg, err := app.GetMessage(id)
	if err != nil {
		return nil, err
	}
	if msg.AuthorID != nil {
		msg.Author, _ = app.GetUserByID(*msg.AuthorID)
	}
	return msg, nil
}

func (app *App) ListChannelMessages(channelID int64, beforeID int64, limit int) ([]*Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var rows *sql.Rows
	var err error
	if beforeID > 0 {
		rows, err = app.DB.Query(`
			SELECT id, channel_id, author_id, content, content_html, is_ai, is_system, created_at, edited_at
			FROM messages
			WHERE channel_id = $1 AND id < $2
			ORDER BY created_at DESC, id DESC
			LIMIT $3
		`, channelID, beforeID, limit)
	} else {
		rows, err = app.DB.Query(`
			SELECT id, channel_id, author_id, content, content_html, is_ai, is_system, created_at, edited_at
			FROM messages
			WHERE channel_id = $1
			ORDER BY created_at DESC, id DESC
			LIMIT $3
		`, channelID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return app.scanMessages(rows)
}

func (app *App) ListChannelMessagesWithAuthors(channelID int64, beforeID int64, limit int) ([]*Message, error) {
	messages, err := app.ListChannelMessages(channelID, beforeID, limit)
	if err != nil {
		return nil, err
	}

	// Batch load authors
	authorIDs := map[int64]bool{}
	for _, m := range messages {
		if m.AuthorID != nil {
			authorIDs[*m.AuthorID] = true
		}
	}
	authorCache := map[int64]*User{}
	for id := range authorIDs {
		if u, err := app.GetUserByID(id); err == nil {
			authorCache[id] = u
		}
	}
	for _, m := range messages {
		if m.AuthorID != nil {
			m.Author = authorCache[*m.AuthorID]
		}
	}

	// Reverse to chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (app *App) ListMessageReplies(messageID int64) ([]*Message, error) {
	rows, err := app.DB.Query(`
		SELECT id, channel_id, author_id, content, content_html, is_ai, is_system, created_at, edited_at
		FROM messages
		WHERE parent_message_id = $1
		ORDER BY created_at ASC
	`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return app.scanMessages(rows)
}

func (app *App) EditMessage(id, userID int64, content, contentHTML string) error {
	res, err := app.DB.Exec(`
		UPDATE messages SET content = $1, content_html = $2, edited_at = $3
		WHERE id = $4 AND author_id = $5
	`, content, contentHTML, time.Now(), id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("message not found or not owned by user")
	}
	return nil
}

func (app *App) DeleteMessage(id, userID int64) error {
	res, err := app.DB.Exec(`
		DELETE FROM messages WHERE id = $1 AND author_id = $2
	`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("message not found or not owned by user")
	}
	return nil
}

func (app *App) ToggleReaction(messageID, userID int64, reaction string) error {
	var count int
	err := app.DB.QueryRow(`
		SELECT COUNT(*) FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND reaction = $3
	`, messageID, userID, reaction).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		_, err = app.DB.Exec(`
			DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND reaction = $3
		`, messageID, userID, reaction)
	} else {
		_, err = app.DB.Exec(`
			INSERT INTO message_reactions (message_id, user_id, reaction) VALUES ($1, $2, $3)
		`, messageID, userID, reaction)
	}
	return err
}

func (app *App) GetMessageReactions(messageID int64) ([]Reaction, error) {
	rows, err := app.DB.Query(`
		SELECT mr.user_id, u.username, mr.reaction, mr.created_at
		FROM message_reactions mr
		JOIN users u ON u.id = mr.user_id
		WHERE mr.message_id = $1
		ORDER BY mr.created_at
	`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []Reaction
	for rows.Next() {
		r := Reaction{}
		if err := rows.Scan(&r.UserID, &r.Username, &r.Reaction, &r.CreatedAt); err != nil {
			return nil, err
		}
		reactions = append(reactions, r)
	}
	return reactions, rows.Err()
}

func (app *App) UpdateChannelUnread(channelID, userID int64, lastMessageID int64) error {
	_, err := app.DB.Exec(`
		INSERT INTO channel_unreads (user_id, channel_id, last_read_message_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, channel_id) DO UPDATE SET last_read_message_id = $3
	`, userID, channelID, lastMessageID)
	return err
}

// --- AI Agents ---

func (app *App) CreateAIAgent(workspaceID int64, name, agentType, systemPrompt string) (*AIAgent, error) {
	agent := &AIAgent{}
	err := app.DB.QueryRow(`
		INSERT INTO ai_agents (workspace_id, name, type, system_prompt)
		VALUES ($1, $2, $3, $4)
		RETURNING id, workspace_id, name, type, system_prompt, enabled, created_at
	`, workspaceID, name, agentType, systemPrompt).Scan(
		&agent.ID, &agent.WorkspaceID, &agent.Name, &agent.Type, &agent.SystemPrompt,
		&agent.Enabled, &agent.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

func (app *App) GetAIAgent(id int64) (*AIAgent, error) {
	agent := &AIAgent{}
	err := app.DB.QueryRow(`
		SELECT id, workspace_id, name, type, system_prompt, enabled, created_at
		FROM ai_agents WHERE id = $1
	`, id).Scan(&agent.ID, &agent.WorkspaceID, &agent.Name, &agent.Type, &agent.SystemPrompt,
		&agent.Enabled, &agent.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

func (app *App) ListWorkspaceAgents(workspaceID int64) ([]*AIAgent, error) {
	rows, err := app.DB.Query(`
		SELECT id, workspace_id, name, type, system_prompt, enabled, created_at
		FROM ai_agents
		WHERE workspace_id = $1
		ORDER BY name
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*AIAgent
	for rows.Next() {
		agent := &AIAgent{}
		if err := rows.Scan(&agent.ID, &agent.WorkspaceID, &agent.Name, &agent.Type, &agent.SystemPrompt,
			&agent.Enabled, &agent.CreatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func (app *App) UpdateAIAgent(id int64, systemPrompt string, enabled bool) error {
	_, err := app.DB.Exec(`
		UPDATE ai_agents SET system_prompt = $1, enabled = $2 WHERE id = $3
	`, systemPrompt, enabled, id)
	return err
}

// --- Helpers ---

func (app *App) scanMessages(rows *sql.Rows) ([]*Message, error) {
	var messages []*Message
	for rows.Next() {
		msg := &Message{}
		var authorID sql.NullInt64
		var editedAt sql.NullTime
		if err := rows.Scan(&msg.ID, &msg.ChannelID, &authorID, &msg.Content, &msg.ContentHTML,
			&msg.IsAI, &msg.IsSystem, &msg.CreatedAt, &editedAt); err != nil {
			return nil, err
		}
		if authorID.Valid {
			msg.AuthorID = &authorID.Int64
		}
		if editedAt.Valid {
			msg.EditedAt = &editedAt.Time
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

func buildPlaceholders(n int) string {
	placeholders := make([]string, n)
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(placeholders, ",")
}
