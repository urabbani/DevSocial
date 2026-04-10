package claw

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Client manages a claw-code subprocess for a workspace.
type Client struct {
	mu         sync.Mutex
	workspaceID int64
	binPath    string
	workDir    string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout      io.ReadCloser
	running    bool
	lastUsed   time.Time
	cancel     context.CancelFunc
}

// Manager manages claw-code subprocesses across workspaces.
type Manager struct {
	mu       sync.Mutex
	clients  map[int64]*Client
	binPath  string
	dataDir  string
}

// NewManager creates a claw-code manager.
func NewManager(binPath, dataDir string) *Manager {
	return &Manager{
	clients: make(map[int64]*Client),
		binPath: binPath,
		dataDir: dataDir,
	}
}

// BinPath returns the configured binary path.
func (m *Manager) BinPath() string {
	return m.binPath
}

// GetOrCreate returns the client for a workspace, starting it if needed.
func (m *Manager) GetOrCreate(workspaceID int64) *Client {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[workspaceID]; ok {
		client.lastUsed = time.Now()
		return client
	}

	workDir := fmt.Sprintf("%s/workspace-%d", m.dataDir, workspaceID)
	os.MkdirAll(workDir, 0755)

	client := &Client{
		workspaceID: workspaceID,
		binPath:    m.binPath,
		workDir:    workDir,
		running:    false,
	}
	m.clients[workspaceID] = client

	return client
}

// SendMessage sends a prompt to claw-code and streams the response.
type StreamEvent struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Text    string `json:"text,omitempty"`
	Name    string `json:"name,omitempty"`
	Input   string `json:"input,omitempty"`
	Error   string `json:"error,omitempty"`
}

// SendMessage sends a prompt and returns the full response text.
func (c *Client) SendMessage(ctx context.Context, prompt string, onChunk func(text string)) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Restart if not running or crashed
	if !c.running {
		if err := c.start(); err != nil {
			return "", fmt.Errorf("failed to start claw-code: %w", err)
		}
	}

	c.lastUsed = time.Now()

	// Write prompt to stdin as NDJSON
	msg := map[string]string{
		"type":    "message",
		"content": prompt,
	}
	data, _ := json.Marshal(msg)
	c.stdin.Write(data)
	c.stdin.Write([]byte("\n"))

	// Read response events from stdout
	var fullResponse strings.Builder
	scanner := bufio.NewScanner(c.stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "{") {
			continue
		}

		var evt StreamEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}

		switch evt.Type {
		case "assistant":
			// Text delta
			if evt.Text != "" {
				if onChunk != nil {
					onChunk(evt.Text)
				}
				fullResponse.WriteString(evt.Text)
			}
		case "result":
			// Final result
			if evt.Content != "" {
				fullResponse.WriteString(evt.Content)
			}
			return fullResponse.String(), nil
		case "error":
			return fullResponse.String(), fmt.Errorf("claw-code error: %s", evt.Error)
		}
	}

	// Scanner ended without "result" — process likely crashed
	c.running = false
	if fullResponse.Len() > 0 {
		return fullResponse.String(), nil
	}
	return "", fmt.Errorf("claw-code process ended unexpectedly")
}

// Stop terminates the claw-code process.
func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	if c.cancel != nil {
		c.cancel()
	}
	c.stdin.Close()
	c.stdout.Close()
	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	c.running = false
}

// start launches the claw-code subprocess.
func (c *Client) start() error {
	if c.running {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	args := []string{
		c.binPath,
		"--output-format", "json",
		"--resume", fmt.Sprintf("workspace-%d", c.workspaceID),
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = c.workDir
	cmd.Env = append(os.Environ(),
		"CLAUDE_CODE=1",
	)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdinPipe
	c.stdout = stdoutPipe
	c.running = true
	c.lastUsed = time.Now()

	// Log stderr for debugging
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			log.Printf("[claw-code] stderr: %s", scanner.Text())
		}
	}()

	// Monitor process exit
	go func() {
		err := cmd.Wait()
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
		if err != nil {
			log.Printf("[claw-code] process exited: %v", err)
		}
	}()

	return nil
}

// IsAvailable returns true if claw-code binary exists.
func IsAvailable(binPath string) bool {
	_, err := os.Stat(binPath)
	return err == nil
}

// FindBinary searches common paths for the claw binary.
func FindBinary() string {
	paths := []string{
		"claw",
		"/usr/local/bin/claw",
	"/opt/claw-code/claw",
	"./claw",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return "claw" // fallback
}

// BuildPrompt creates a prompt for claw-code from chat context.
func BuildPrompt(channelName string, messages []ChatMessage, systemPrompt string) string {
	var sb strings.Builder

	if systemPrompt != "" {
		sb.WriteString(systemPrompt)
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("You are an AI assistant in the #%s channel of DevSocial.\n\n", channelName))
	sb.WriteString("Here is the recent conversation for context:\n\n")

	// Include last N messages as context
	maxMessages := 50
	start := 0
	if len(messages) > maxMessages {
		start = len(messages) - maxMessages
	}
	for _, msg := range messages[start:] {
		author := "AI"
		if !msg.IsAI {
			author = msg.AuthorUsername
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", author, msg.Content))
	}

	sb.WriteString("\nBased on the conversation above, fulfill the user's request. Respond concisely.")

	return sb.String()
}

// ChatMessage represents a message from the chat for context.
type ChatMessage struct {
	ID             int64  `json:"id"`
	Content        string `json:"content"`
	IsAI           bool   `json:"is_ai"`
	AuthorUsername string `json:"author_username"`
	CreatedAt      string `json:"created_at"`
}
