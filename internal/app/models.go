package app

import "time"

// --- User & Auth ---

type User struct {
	ID          int64     `json:"id"`
	GitHubID    int64     `json:"github_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	Bio         string    `json:"bio,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// --- Workspace ---

type Workspace struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Slug        string    `json:"slug"`
	CreatedAt   time.Time `json:"created_at"`
	// Computed
	MemberCount int  `json:"member_count"`
	IsMember    bool `json:"is_member"`
}

// --- Channel ---

type Channel struct {
	ID          int64     `json:"id"`
	WorkspaceID int64     `json:"workspace_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        string    `json:"type"` // "text" or "ai"
	Position    int       `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
	// Computed
	UnreadCount int `json:"unread_count"`
}

// --- Message ---

type Message struct {
	ID               int64      `json:"id"`
	ChannelID        int64      `json:"channel_id"`
	AuthorID         *int64     `json:"author_id,omitempty"`
	ParentMessageID  *int64     `json:"parent_message_id,omitempty"`
	Content          string     `json:"content"`
	ContentHTML      string     `json:"content_html,omitempty"`
	IsAI             bool       `json:"is_ai"`
	IsSystem         bool       `json:"is_system"`
	CreatedAt        time.Time  `json:"created_at"`
	EditedAt         *time.Time `json:"edited_at,omitempty"`
	// Joined fields
	Author    *User    `json:"author,omitempty"`
	Reactions []Reaction `json:"reactions,omitempty"`
	Replies   []*Message `json:"replies,omitempty"`
}

type Reaction struct {
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	Reaction  string    `json:"reaction"`
	CreatedAt time.Time `json:"created_at"`
}

// --- AI Agent ---

type AIAgent struct {
	ID           int64     `json:"id"`
	WorkspaceID  int64     `json:"workspace_id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	SystemPrompt string    `json:"system_prompt"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
}

// --- Workspace Member ---

type WorkspaceMember struct {
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}
