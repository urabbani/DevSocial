package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"devsocial/internal/claw"
)

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- Current User ---

func (app *App) handleGetMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	writeJSON(w, http.StatusOK, user)
}

// --- Workspaces ---

func (app *App) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	workspaces, err := app.ListUserWorkspaces(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workspaces")
		return
	}
	writeJSON(w, http.StatusOK, workspaces)
}

func (app *App) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Slug        string `json:"slug"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if input.Slug == "" {
		input.Slug = strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
	}

	ws, err := app.CreateWorkspace(input.Name, input.Description, input.Slug, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workspace")
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

func (app *App) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	ws, err := app.GetWorkspace(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (app *App) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := app.UpdateWorkspace(id, user.ID, input.Name, input.Description); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (app *App) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	if err := app.DeleteWorkspace(id, user.ID); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Members ---

func (app *App) handleListMembers(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	members, err := app.ListWorkspaceMembers(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (app *App) handleAddMember(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	var input struct {
		UserID int64  `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.UserID == 0 {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if err := app.AddWorkspaceMember(workspaceID, input.UserID, "member"); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add member")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})
}

func (app *App) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	targetUserID, err := strconv.ParseInt(r.PathValue("uid"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := app.RemoveWorkspaceMember(workspaceID, targetUserID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// --- Channels ---

func (app *App) handleListChannels(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	channels, err := app.ListWorkspaceChannels(id, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}
	writeJSON(w, http.StatusOK, channels)
}

func (app *App) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Position    int    `json:"position"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if input.Type == "" {
		input.Type = "text"
	}

	ch, err := app.CreateChannel(id, input.Name, input.Description, input.Type, input.Position)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create channel")
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

func (app *App) handleGetChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	ch, err := app.GetChannel(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "channel not found")
		return
	}
	writeJSON(w, http.StatusOK, ch)
}

func (app *App) handleUpdateChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := app.UpdateChannel(id, input.Name, input.Description); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update channel")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (app *App) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	if err := app.DeleteChannel(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete channel")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Messages ---

func (app *App) handleCreateMessage(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	channelID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	var input struct {
		Content string `json:"content"`
		IsAI     bool   `json:"is_ai,omitempty"`
	}
	if err := readJSON(r, &input); err != nil || input.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	contentHTML, err := RenderMarkdown(input.Content)
	if err != nil {
		contentHTML = input.Content
	}

	var authorID *int64
	if !input.IsAI {
		authorID = &user.ID
	}

	msg, err := app.CreateMessage(channelID, authorID, input.Content, contentHTML, input.IsAI, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create message")
		return
	}

	// Load author for broadcast
	if msg.AuthorID != nil {
		msg.Author, _ = app.GetUserByID(*msg.AuthorID)
	}

	// Broadcast to all WebSocket subscribers in the channel
	msgBytes, _ := json.Marshal(WSMessageOut{
		Type:    "message",
		Message: msg,
	})
	app.Hub.BroadcastToChannel(channelID, msgBytes)

	// If this is an AI channel and the message @mentions an agent, trigger claw-code
	if !input.IsAI && app.isAIChannel(channelID) {
		go app.processAIClaim(channelID, msg)
	}

	writeJSON(w, http.StatusCreated, msg)
}

func (app *App) handleListMessages(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	beforeID, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	messages, err := app.ListChannelMessagesWithAuthors(channelID, beforeID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list messages")
		return
	}
	if messages == nil {
		messages = []*Message{}
	}

	// Update unread marker
	user := UserFromContext(r)
	if len(messages) > 0 {
		lastID := messages[len(messages)-1].ID
		_ = app.UpdateChannelUnread(channelID, user.ID, lastID)
	}

	writeJSON(w, http.StatusOK, messages)
}

func (app *App) handleGetMessage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message id")
		return
	}
	msg, err := app.GetMessageWithAuthor(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	writeJSON(w, http.StatusOK, msg)
}

func (app *App) handleEditMessage(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message id")
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	contentHTML, err := RenderMarkdown(input.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to render markdown")
		return
	}

	if err := app.EditMessage(id, user.ID, input.Content, contentHTML); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (app *App) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message id")
		return
	}
	if err := app.DeleteMessage(id, user.ID); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Reactions ---

func (app *App) handleToggleReaction(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message id")
		return
	}
	var input struct {
		Reaction string `json:"reaction"`
	}
	if err := readJSON(r, &input); err != nil || input.Reaction == "" {
		writeError(w, http.StatusBadRequest, "reaction is required")
		return
	}
	if err := app.ToggleReaction(id, user.ID, input.Reaction); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to toggle reaction")
		return
	}
	reactions, _ := app.GetMessageReactions(id)
	writeJSON(w, http.StatusOK, reactions)
}

// --- AI Agents ---

func (app *App) handleListAgents(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	agents, err := app.ListWorkspaceAgents(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agents")
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (app *App) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	var input struct {
		Name         string `json:"name"`
		Type         string `json:"type"`
		SystemPrompt string `json:"system_prompt"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if input.Type == "" {
		input.Type = "claw-code"
	}

	agent, err := app.CreateAIAgent(workspaceID, input.Name, input.Type, input.SystemPrompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create agent")
		return
	}
	writeJSON(w, http.StatusCreated, agent)
}

func (app *App) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	var input struct {
		SystemPrompt string `json:"system_prompt"`
		Enabled      *bool  `json:"enabled"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Enabled == nil {
		enabled := true
		input.Enabled = &enabled
	}
	if err := app.UpdateAIAgent(id, input.SystemPrompt, *input.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update agent")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// --- AI Processing ---

// WSMessageOut is the format sent to WebSocket clients.
type WSMessageOut struct {
	Type      string   `json:"type"`
	Message   *Message `json:"message,omitempty"`
	Text      string   `json:"text,omitempty"`
	MessageID int64    `json:"message_id,omitempty"`
	ChannelID int64    `json:"channel_id,omitempty"`
}

// isAIChannel checks if a channel is an AI channel.
func (app *App) isAIChannel(channelID int64) bool {
	ch, err := app.GetChannel(channelID)
	if err != nil {
		return false
	}
	return ch.Type == "ai"
}

// processAIClaim runs claw-code in a goroutine and streams the response.
func (app *App) processAIClaim(channelID int64, userMsg *Message) {
	if !claw.IsAvailable(app.Claw.BinPath()) {
		// Post a system message that AI is not available
		sysContent := "AI assistant is not available. Check that claw-code is installed and the CLAW_BIN_PATH env var is set."
		html, _ := RenderMarkdown(sysContent)
		sysMsg, _ := app.CreateMessage(channelID, nil, sysContent, html, false, true)
		if sysMsg != nil {
			data, _ := json.Marshal(WSMessageOut{Type: "message", Message: sysMsg})
			app.Hub.BroadcastToChannel(channelID, data)
		}
		return
	}

	// Gather recent chat context
	messages, _ := app.ListChannelMessages(channelID, 0, 50)

	// Build chat messages for context
	var chatMessages []claw.ChatMessage
	for _, m := range messages {
		cm := claw.ChatMessage{
			ID:             m.ID,
			Content:        m.Content,
			IsAI:           m.IsAI,
			AuthorUsername: "user",
		}
		if m.Author != nil {
			cm.AuthorUsername = m.Author.Username
		}
		chatMessages = append(chatMessages, cm)
	}

	// Get workspace ID from channel
	ch, _ := app.GetChannel(channelID)
	workspaceID := int64(0)
	if ch != nil {
		workspaceID = ch.WorkspaceID
	}

	// Get system prompt from agents
	agents, _ := app.ListWorkspaceAgents(workspaceID)
	systemPrompt := ""
	if len(agents) > 0 {
		systemPrompt = agents[0].SystemPrompt
	}

	// Build the prompt for claw-code
	chName := "general"
	if ch != nil {
		chName = ch.Name
	}
	prompt := claw.BuildPrompt(chName, chatMessages, systemPrompt)

	// Create a system message that AI is thinking
	thinkingContent := "AI is processing your request..."
	thinkingHTML, _ := RenderMarkdown(thinkingContent)
	thinkingMsg, _ := app.CreateMessage(channelID, nil, thinkingContent, thinkingHTML, true, true)
	if thinkingMsg != nil {
		data, _ := json.Marshal(WSMessageOut{Type: "message", Message: thinkingMsg})
		app.Hub.BroadcastToChannel(channelID, data)
	}

	// Send prompt to claw-code
	client := app.Claw.GetOrCreate(workspaceID)
	ctx := context.Background()

	// Stream response chunks back to the channel
	var aiContent strings.Builder
	_, sendErr := client.SendMessage(ctx, prompt, func(chunk string) {
		chunkData, _ := json.Marshal(WSMessageOut{
			Type:      "ai_chunk",
			ChannelID: channelID,
			Text:      chunk,
		})
		app.Hub.BroadcastToChannel(channelID, chunkData)
		aiContent.WriteString(chunk)
	})

	if sendErr != nil {
		errContent := fmt.Sprintf("AI error: %v", sendErr)
		errHTML, _ := RenderMarkdown(errContent)
		errMsg, _ := app.CreateMessage(channelID, nil, errContent, errHTML, true, true)
		if errMsg != nil {
			data, _ := json.Marshal(WSMessageOut{Type: "message", Message: errMsg})
			app.Hub.BroadcastToChannel(channelID, data)
		}
		return
	}

	// Save the complete AI response as a real message
	if aiContent.Len() > 0 {
		responseContent := aiContent.String()
		responseHTML, _ := RenderMarkdown(responseContent)
		aiMsg, createErr := app.CreateMessage(channelID, nil, responseContent, responseHTML, true, false)
		if createErr == nil {
			data, _ := json.Marshal(WSMessageOut{Type: "message", Message: aiMsg})
			app.Hub.BroadcastToChannel(channelID, data)
		}
	}

	// Delete the "thinking" placeholder message
	if thinkingMsg != nil {
		app.DeleteMessage(thinkingMsg.ID, 0) // author_id=0 bypasses ownership check
		delData, _ := json.Marshal(WSMessageOut{
			Type:      "message_delete",
			MessageID: thinkingMsg.ID,
		})
		app.Hub.BroadcastToChannel(channelID, delData)
	}
}

// --- Upload ---

func (app *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	// TODO: implement image upload (reuse existing logic from old handlers.go)
	writeError(w, http.StatusNotImplemented, "upload not yet implemented")
	_ = fmt.Sprintf("upload handler placeholder")
}

// --- WebSocket ---

func (app *App) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	app.Hub.ServeWS(w, r, user.ID)
}
