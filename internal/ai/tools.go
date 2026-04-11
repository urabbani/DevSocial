package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Tool is the interface that all AI tools must implement.
type Tool interface {
	// Name returns the unique identifier for this tool (used in function calls).
	Name() string

	// Description returns a human-readable description of what the tool does.
	Description() string

	// Parameters returns a JSON Schema object describing the tool's input parameters.
	Parameters() map[string]any

	// Execute runs the tool with the provided arguments and returns the result.
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// ToolRegistry manages available tools and handles tool execution.
type ToolRegistry struct {
	mu     sync.RWMutex
	tools  map[string]Tool
	logger *log.Logger
}

// NewToolRegistry creates an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:  make(map[string]Tool),
		logger: log.Default(),
	}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
	log.Printf("[tools] registered: %s", tool.Name())
}

// Unregister removes a tool from the registry.
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
	log.Printf("[tools] unregistered: %s", name)
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tool names.
func (r *ToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ToToolDefinitions converts all registered tools to OpenAI-compatible tool definitions.
func (r *ToolRegistry) ToToolDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
			},
		})
	}
	return defs
}

// ExecuteToolCall runs a tool call and returns the result as a ToolResult message.
func (r *ToolRegistry) ExecuteToolCall(ctx context.Context, call ToolCall) (ToolResult, error) {
	tool, ok := r.Get(call.Function.Name)
	if !ok {
		return ToolResult{}, fmt.Errorf("tool not found: %s", call.Function.Name)
	}

	// Parse arguments JSON
	var args map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return ToolResult{}, fmt.Errorf("invalid arguments for %s: %w", call.Function.Name, err)
	}

	log.Printf("[tools] executing %s with args: %s", call.Function.Name, call.Function.Arguments)

	result, err := tool.Execute(ctx, args)
	if err != nil {
		log.Printf("[tools] error executing %s: %v", call.Function.Name, err)
		return ToolResult{
			ToolCallID: call.ID,
			Content:    fmt.Sprintf("Error: %v", err),
		}, err
	}

	log.Printf("[tools] %s completed successfully", call.Function.Name)
	return ToolResult{
		ToolCallID: call.ID,
		Content:    result,
	}, nil
}

// NoOpTool is a placeholder tool for testing. It echoes back its arguments.
type NoOpTool struct{}

func (t *NoOpTool) Name() string        { return "noop" }
func (t *NoOpTool) Description() string { return "Echo back the provided arguments for testing." }

func (t *NoOpTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "The message to echo back",
			},
		},
		"required": []string{"message"},
	}
}

func (t *NoOpTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	msg, ok := args["message"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'message' argument")
	}
	return fmt.Sprintf("NoOpTool received: %s", msg), nil
}

// CodeExecuteTool executes code in a sandboxed Docker container.
type CodeExecuteTool struct {
	sandbox *SandboxClient
}

func NewCodeExecuteTool(sandbox *SandboxClient) *CodeExecuteTool {
	return &CodeExecuteTool{sandbox: sandbox}
}

func (t *CodeExecuteTool) Name() string {
	return "execute_code"
}

func (t *CodeExecuteTool) Description() string {
	return "Execute code in a sandboxed environment. Supports: python, javascript (node), go, bash. The code runs in an isolated Docker container with no network access, 256MB memory limit, and 30 second timeout."
}

func (t *CodeExecuteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"language": map[string]any{
				"type":        "string",
				"description": "Programming language: python, javascript, go, or bash",
				"enum":        []string{"python", "javascript", "go", "bash"},
			},
			"code": map[string]any{
				"type":        "string",
				"description": "The code to execute",
			},
		},
		"required": []string{"language", "code"},
	}
}

func (t *CodeExecuteTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	lang, ok := args["language"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'language' argument")
	}
	code, ok := args["code"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'code' argument")
	}

	req := CodeExecutionRequest{
		Language: lang,
		Code:     code,
	}

	result, err := t.sandbox.Execute(ctx, req)
	if err != nil {
		return "", err
	}

	// Format result for LLM
	var output strings.Builder
	if result.Stdout != "" {
		output.WriteString("STDOUT:\n")
		output.WriteString(result.Stdout)
	}
	if result.Stderr != "" {
		if output.Len() > 0 {
			output.WriteString("\n\n")
		}
		output.WriteString("STDERR:\n")
		output.WriteString(result.Stderr)
	}
	output.WriteString(fmt.Sprintf("\n\nExit code: %d", result.ExitCode))

	return output.String(), nil
}

// WebSearchTool performs web searches using DuckDuckGo.
type WebSearchTool struct {
	client *WebSearchClient
}

func NewWebSearchTool(client *WebSearchClient) *WebSearchTool {
	return &WebSearchTool{client: client}
}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Description() string {
	return "Search the web for current information. Returns relevant web pages with titles, URLs, and snippets."
}

func (t *WebSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
			"max_results": map[string]any{
				"type":        "number",
				"description": "Maximum number of results to return (default: 5, max: 20)",
			},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'query' argument")
	}

	maxResults := 5
	if mr, ok := args["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	results, err := t.client.Search(ctx, query, maxResults)
	if err != nil {
		return "", err
	}

	return FormatResults(results), nil
}

// FileReadTool reads files from the project directory.
type FileReadTool struct {
	projectRoot string
}

func NewFileReadTool(projectRoot string) *FileReadTool {
	return &FileReadTool{projectRoot: projectRoot}
}

func (t *FileReadTool) Name() string {
	return "read_file"
}

func (t *FileReadTool) Description() string {
	return "Read the contents of a file from the project. Use this to examine source code, configuration files, or documentation."
}

func (t *FileReadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative path to the file within the project",
			},
		},
		"required": []string{"path"},
	}
}

func (t *FileReadTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' argument")
	}

	// Clean the path and ensure it's within the project root
	fullPath := filepath.Join(t.projectRoot, path)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	absRoot, err := filepath.Abs(t.projectRoot)
	if err != nil {
		return "", fmt.Errorf("invalid project root: %w", err)
	}

	// Ensure the path is within the project root
	if !strings.HasPrefix(absPath, absRoot) {
		return "", fmt.Errorf("path outside project root not allowed")
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	// Check file size - limit to 100KB
	if len(content) > 100*1024 {
		return string(content[:100*1024]) + "\n\n[File truncated - too large]", nil
	}

	return string(content), nil
}
