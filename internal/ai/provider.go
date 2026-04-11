package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Provider calls LiteLLM's OpenAI-compatible chat completions API with streaming.
type Provider struct {
	mu       sync.Mutex
	baseURL  string
	apiKey   string
	model    string
	client   *http.Client
}

// NewProvider creates an AI provider that talks to LiteLLM.
func NewProvider() *Provider {
	baseURL := os.Getenv("LITELLM_URL")
	if baseURL == "" {
		baseURL = "http://localhost:4000"
	}
	apiKey := os.Getenv("LITELLM_API_KEY")
	if apiKey == "" {
		apiKey = "sk-devsocial-local"
	}

	return &Provider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client: &http.Client{Timeout: 180 * time.Second},
	}
}

// ChatMessage is a single message in the conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request body sent to LiteLLM.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatResponse is a non-streaming response.
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// SSEChunk is a single Server-Sent Event chunk from the streaming API.
type SSEChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// SetModel updates the active model.
func (p *Provider) SetModel(model string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.model = model
}

// GetModel returns the current model.
func (p *Provider) GetModel() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.model
}

// SendMessage sends a chat completion request and returns the full response.
func (p *Provider) SendMessage(model string, messages []ChatMessage, temperature float64) (string, error) {
	reqBody := ChatRequest{
		Model:       model,
		Messages:    messages,
		Stream:      false,
		Temperature: temperature,
		MaxTokens:   4096,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("litellm error %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// SendMessageStream sends a chat completion request and streams chunks via onChunk callback.
// Returns the full accumulated response text.
func (p *Provider) SendMessageStream(model string, messages []ChatMessage, temperature float64, onChunk func(text string)) (string, error) {
	reqBody := ChatRequest{
		Model:       model,
		Messages:    messages,
		Stream:      true,
		Temperature: temperature,
		MaxTokens:   4096,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("litellm error %d: %s", resp.StatusCode, string(respBody))
	}

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk SSEChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				if onChunk != nil {
					onChunk(choice.Delta.Content)
				}
				fullResponse.WriteString(choice.Delta.Content)
			}
			if choice.FinishReason != nil && *choice.FinishReason == "stop" {
				return fullResponse.String(), nil
			}
		}
	}

	return fullResponse.String(), scanner.Err()
}

// BuildChatMessages creates chat messages from conversation history.
func BuildChatMessages(systemPrompt string, history []HistoryMessage, maxMessages int) []ChatMessage {
	var messages []ChatMessage

	if systemPrompt != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: systemPrompt})
	}

	start := 0
	if len(history) > maxMessages {
		start = len(history) - maxMessages
	}

	for _, msg := range history[start:] {
		role := "user"
		content := msg.Content
		if msg.IsAI {
			role = "assistant"
		} else {
			content = fmt.Sprintf("[%s]: %s", msg.AuthorUsername, msg.Content)
		}
		messages = append(messages, ChatMessage{Role: role, Content: content})
	}

	return messages
}

// HistoryMessage represents a chat message for building context.
type HistoryMessage struct {
	ID             int64
	Content        string
	IsAI           bool
	AuthorUsername string
}

// GetModels fetches available models from LiteLLM.
func (p *Provider) GetModels() ([]string, error) {
	req, err := http.NewRequest("GET", p.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

// GenerateEmbedding generates an embedding vector for the given text.
func (p *Provider) GenerateEmbedding(model string, text string) ([]float32, error) {
	reqBody := map[string]any{
		"model": model,
		"input": text,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", p.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return result.Data[0].Embedding, nil
}

// Health checks if LiteLLM is reachable.
func (p *Provider) Health() error {
	resp, err := p.client.Get(p.baseURL + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("litellm health check failed: %d", resp.StatusCode)
	}
	return nil
}
