package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"devsocial/internal/ai"
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
	}
	if err := readJSON(r, &input); err != nil || input.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	contentHTML, err := RenderMarkdown(input.Content)
	if err != nil {
		contentHTML = input.Content
	}

	msg, err := app.CreateMessage(channelID, &user.ID, input.Content, contentHTML, false, false)
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

	// If this is an AI channel, trigger claw-code
	if app.isAIChannel(channelID) {
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
	ToolCall  *AIToolCall `json:"tool_call,omitempty"`
}

// AIToolCall represents a tool call sent to the client.
type AIToolCall struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Args   string                 `json:"arguments"`
	Status string                 `json:"status"` // pending, executing, completed, error
	Result string                 `json:"result,omitempty"`
}

// isAIChannel checks if a channel is an AI channel.
func (app *App) isAIChannel(channelID int64) bool {
	ch, err := app.GetChannel(channelID)
	if err != nil {
		return false
	}
	return ch.Type == "ai"
}

// processAIClaim runs an AI completion via LiteLLM with tool calling support.
func (app *App) processAIClaim(channelID int64, userMsg *Message) {
	// Check AI provider is available
	if app.AI == nil {
		sysContent := "AI assistant is not available. Check that LiteLLM is configured."
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

	// Build chat history for the AI
	var history []ai.HistoryMessage
	for _, m := range messages {
		hm := ai.HistoryMessage{
			ID:             m.ID,
			Content:        m.Content,
			IsAI:           m.IsAI,
			AuthorUsername: "user",
		}
		if m.Author != nil {
			hm.AuthorUsername = m.Author.Username
		}
		history = append(history, hm)
	}

	// Get workspace and channel info
	ch, _ := app.GetChannel(channelID)
	workspaceID := int64(0)
	chName := "general"
	if ch != nil {
		workspaceID = ch.WorkspaceID
		chName = ch.Name
	}

	// Build system prompt with tool documentation
	agents, _ := app.ListWorkspaceAgents(workspaceID)
	basePrompt := ""
	if len(agents) > 0 && agents[0].SystemPrompt != "" {
		basePrompt = agents[0].SystemPrompt
	}

	// Add tool documentation to system prompt
	toolDocs := app.buildToolDocumentation()
	systemPrompt := basePrompt + fmt.Sprintf(
		"\n\nYou are an AI assistant in the #%s channel of DevSocial, a collaborative platform for developers and researchers. Help the team with coding, data analysis, document review, and problem-solving. Be concise and actionable.%s",
		chName, toolDocs,
	)

	// Load max context setting
	maxMessages := 50
	if val := app.getAISetting("ai_max_context_messages", ""); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			maxMessages = n
		}
	}

	chatMessages := ai.BuildChatMessages(systemPrompt, history, maxMessages)

	// Get temperature from settings
	temperature := 0.7
	if val := app.getAISetting("ai_temperature", ""); val != "" {
		if t, err := strconv.ParseFloat(val, 64); err == nil {
			temperature = t
		}
	}

	// Show "thinking" indicator
	thinkingContent := "AI is processing your request..."
	thinkingHTML, _ := RenderMarkdown(thinkingContent)
	thinkingMsg, _ := app.CreateMessage(channelID, nil, thinkingContent, thinkingHTML, true, true)
	if thinkingMsg != nil {
		data, _ := json.Marshal(WSMessageOut{Type: "message", Message: thinkingMsg})
		app.Hub.BroadcastToChannel(channelID, data)
	}

	// Get available tools
	tools := app.Tools.ToToolDefinitions()

	// Run the tool loop
	model := app.AI.GetModel()
	log.Printf("[ai] starting tool loop for channel %d", channelID)

	maxIterations := 10
	iteration := 0
	var finalResponse strings.Builder

	for iteration < maxIterations {
		iteration++
		log.Printf("[ai] tool loop iteration %d", iteration)

		// Stream response from LiteLLM with tools
		result := app.AI.SendMessageStreamWithTools(model, chatMessages, temperature, tools, func(chunk string) {
			chunkData, _ := json.Marshal(WSMessageOut{
				Type:      "ai_chunk",
				ChannelID: channelID,
				Text:      chunk,
			})
			app.Hub.BroadcastToChannel(channelID, chunkData)
			finalResponse.WriteString(chunk)
		})

		if result.Err != nil {
			log.Printf("[ai] error: %v", result.Err)
			app.handleAIError(channelID, result.Err, thinkingMsg)
			return
		}

		// If no tool calls, we're done
		if len(result.ToolCalls) == 0 {
			log.Printf("[ai] no tool calls, finishing")
			break
		}

		// Process tool calls
		for _, toolCall := range result.ToolCalls {
			log.Printf("[ai] tool call: %s with args: %s", toolCall.Function.Name, toolCall.Function.Arguments)

			// Send tool_call event to client
			toolData, _ := json.Marshal(WSMessageOut{
				Type:      "tool_call",
				ChannelID: channelID,
				ToolCall: &AIToolCall{
					ID:     toolCall.ID,
					Name:   toolCall.Function.Name,
					Args:   toolCall.Function.Arguments,
					Status: "executing",
				},
			})
			app.Hub.BroadcastToChannel(channelID, toolData)

			// Execute the tool
			toolResult, err := app.Tools.ExecuteToolCall(context.Background(), toolCall)

			// Send tool_result event to client
			resultStatus := "completed"
			if err != nil {
				resultStatus = "error"
			}
			resultData, _ := json.Marshal(WSMessageOut{
				Type:      "tool_result",
				ChannelID: channelID,
				ToolCall: &AIToolCall{
					ID:     toolCall.ID,
					Name:   toolCall.Function.Name,
					Status: resultStatus,
					Result: toolResult.Content,
				},
			})
			app.Hub.BroadcastToChannel(channelID, resultData)

			// Append tool result to messages for next iteration
			chatMessages = append(chatMessages, ai.ChatMessage{
				Role:       "assistant",
				ToolCalls:  []ai.ToolCall{toolCall},
			})
			chatMessages = append(chatMessages, ai.ChatMessage{
				Role:       "tool",
				ToolCallID: toolResult.ToolCallID,
				Content:    toolResult.Content,
			})
		}
	}

	// Clear the streaming placeholder
	clearData, _ := json.Marshal(WSMessageOut{
		Type:      "ai_chunk",
		ChannelID: channelID,
		Text:      "",
	})
	app.Hub.BroadcastToChannel(channelID, clearData)

	// Signal stream done
	doneData, _ := json.Marshal(WSMessageOut{
		Type:      "ai_stream_done",
		ChannelID: channelID,
	})
	app.Hub.BroadcastToChannel(channelID, doneData)

	// Save the complete AI response as a real message
	responseContent := finalResponse.String()
	if responseContent == "" {
		responseContent = "AI returned an empty response."
	}

	responseHTML, _ := RenderMarkdown(responseContent)
	aiMsg, createErr := app.CreateMessage(channelID, nil, responseContent, responseHTML, true, false)
	if createErr == nil {
		data, _ := json.Marshal(WSMessageOut{Type: "message", Message: aiMsg})
		app.Hub.BroadcastToChannel(channelID, data)
	}

	// Delete the "thinking" placeholder message
	if thinkingMsg != nil {
		app.DB.Exec(`DELETE FROM messages WHERE id = $1`, thinkingMsg.ID)
		delData, _ := json.Marshal(WSMessageOut{Type: "message_delete", MessageID: thinkingMsg.ID})
		app.Hub.BroadcastToChannel(channelID, delData)
	}
}

// buildToolDocumentation creates a description of available tools for the system prompt.
func (app *App) buildToolDocumentation() string {
	if app.Tools == nil {
		return ""
	}

	tools := app.Tools.List()
	if len(tools) == 0 {
		return ""
	}

	var doc strings.Builder
	doc.WriteString("\n\nAvailable tools:\n")
	for _, name := range tools {
		tool, ok := app.Tools.Get(name)
		if ok {
			doc.WriteString(fmt.Sprintf("- %s: %s\n", name, tool.Description()))
		}
	}

	// Add chart rendering instructions
	doc.WriteString("\n\nWhen displaying data visualizations, use the following format:\n")
	doc.WriteString("```chart\n{ JSON chart configuration for ECharts }\n```\n")
	doc.WriteString("Supported chart types: bar, line, pie, scatter, etc.\n")

	return doc.String()
}

// handleAIError sends an error message and cleans up the thinking indicator.
func (app *App) handleAIError(channelID int64, err error, thinkingMsg *Message) {
	errContent := fmt.Sprintf("AI error: %v", err)
	errHTML, _ := RenderMarkdown(errContent)
	errMsg, _ := app.CreateMessage(channelID, nil, errContent, errHTML, true, true)
	if errMsg != nil {
		data, _ := json.Marshal(WSMessageOut{Type: "message", Message: errMsg})
		app.Hub.BroadcastToChannel(channelID, data)
	}

	// Send stream done event
	doneData, _ := json.Marshal(WSMessageOut{
		Type:      "ai_stream_done",
		ChannelID: channelID,
	})
	app.Hub.BroadcastToChannel(channelID, doneData)

	// Delete thinking message
	if thinkingMsg != nil {
		app.DB.Exec(`DELETE FROM messages WHERE id = $1`, thinkingMsg.ID)
		delData, _ := json.Marshal(WSMessageOut{Type: "message_delete", MessageID: thinkingMsg.ID})
		app.Hub.BroadcastToChannel(channelID, delData)
	}
}

// --- WebSocket ---

func (app *App) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	app.Hub.ServeWS(w, r, user.ID)
}
