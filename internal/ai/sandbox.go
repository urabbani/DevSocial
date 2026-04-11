package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	dockerSocket = "/var/run/docker.sock"
	maxExecution = 30 * time.Second
	maxMemory    = "256m"
	maxPids      = 1024
)

// CodeExecutionRequest contains code to execute in a sandboxed container.
type CodeExecutionRequest struct {
	Language string `json:"language"` // python, javascript, go, bash
	Code     string `json:"code"`
}

// CodeExecutionResult contains the execution output.
type CodeExecutionResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Duration string `json:"duration"`
}

// SandboxClient executes code in Docker containers via HTTP API to mounted socket.
type SandboxClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewSandboxClient creates a new sandbox execution client.
func NewSandboxClient() *SandboxClient {
	return &SandboxClient{
		httpClient: &http.Client{
			Timeout: maxExecution + 5*time.Second,
		},
		baseURL: "http://localhost", // Docker socket is mounted locally
	}
}

// Execute runs code in a sandboxed Docker container.
func (s *SandboxClient) Execute(ctx context.Context, req CodeExecutionRequest) (CodeExecutionResult, error) {
	result := CodeExecutionResult{}

	// Map language to Docker image
	image, err := s.imageForLanguage(req.Language)
	if err != nil {
		return result, err
	}

	// Create container with strict limits
	containerID, err := s.createContainer(ctx, image, req.Code)
	if err != nil {
		return result, fmt.Errorf("create container: %w", err)
	}

	// Clean up container
	defer s.removeContainer(context.Background(), containerID)

	// Start container
	if err := s.startContainer(ctx, containerID); err != nil {
		return result, fmt.Errorf("start container: %w", err)
	}

	// Wait for completion with timeout
	ctx, cancel := context.WithTimeout(ctx, maxExecution)
	defer cancel()

	stdout, stderr, exitCode, err := s.waitContainer(ctx, containerID)
	result.Stdout = stdout
	result.Stderr = stderr
	result.ExitCode = exitCode

	if err != nil && exitCode == 0 {
		// If exit code is 0 but we got an error, it's likely a timeout
		result.Stderr = fmt.Sprintf("Execution timeout or error: %v", err)
	}

	log.Printf("[sandbox] executed %s in container %s: exit=%d", req.Language, containerID, exitCode)
	return result, nil
}

// imageForLanguage maps a language name to a Docker image.
func (s *SandboxClient) imageForLanguage(lang string) (string, error) {
	switch lang {
	case "python", "py":
		return "python:3.12-slim", nil
	case "javascript", "js", "node":
		return "node:20-slim", nil
	case "go", "golang":
		return "golang:1.22-alpine", nil
	case "bash", "sh":
		return "bash:5", nil
	default:
		return "", fmt.Errorf("unsupported language: %s", lang)
	}
}

// createContainer creates a new container with the code to execute.
func (s *SandboxClient) createContainer(ctx context.Context, image, code string) (string, error) {
	// Build container config
	config := map[string]any{
		"Image": image,
		"Cmd":    s.cmdForImage(image, code),
		"Tty":    false,
		"OpenStdin": false,
		"HostConfig": map[string]any{
			"NetworkMode": "none",          // No network access
			"Memory":      256 * 1024 * 1024, // 256MB
			"PidsLimit":   maxPids,
			"ReadOnlyRootfs": true,         // Read-only filesystem
			"AutoRemove":     false,        // We'll remove manually
		},
	}

	body, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		s.baseURL+"/v1.44/containers/create",
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("docker api call failed: %w (is docker socket mounted at %s?)", err, dockerSocket)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("docker create failed: %s", string(respBody))
	}

	var createResp struct {
		ID string `json:"Id"`
		Warnings []string `json:"Warnings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", err
	}

	return createResp.ID, nil
}

// cmdForImage returns the command to run for a given image and code.
func (s *SandboxClient) cmdForImage(image, code string) []string {
	switch {
	case image == "python:3.12-slim":
		return []string{"python", "-c", code}
	case image == "node:20-slim":
		return []string{"node", "-e", code}
	case image == "golang:1.22-alpine":
		// For Go, we need to write a file and run it
		// This is complex, so we'll use a simpler approach: echo to a temp file and run
		return []string{"sh", "-c", fmt.Sprintf("echo '%s' > /tmp/main.go && go run /tmp/main.go", code)}
	case image == "bash:5":
		return []string{"sh", "-c", code}
	default:
		return []string{"sh", "-c", code}
	}
}

// startContainer starts a container.
func (s *SandboxClient) startContainer(ctx context.Context, containerID string) error {
	req, err := http.NewRequestWithContext(ctx, "POST",
		s.baseURL+"/v1.44/containers/"+containerID+"/start",
		nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker start failed: %s", string(respBody))
	}

	return nil
}

// waitContainer waits for a container to finish and returns output.
func (s *SandboxClient) waitContainer(ctx context.Context, containerID string) (stdout, stderr string, exitCode int, err error) {
	req, err := http.NewRequestWithContext(ctx, "POST",
		s.baseURL+"/v1.44/containers/"+containerID+"/wait",
		nil)
	if err != nil {
		return "", "", 0, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()

	var waitResult struct {
		StatusCode int `json:"StatusCode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&waitResult); err != nil {
		return "", "", 0, err
	}

	// Get logs
	stdout, stderr = s.getLogs(ctx, containerID)

	return stdout, stderr, waitResult.StatusCode, nil
}

// getLogs fetches stdout/stderr from a container.
func (s *SandboxClient) getLogs(ctx context.Context, containerID string) (stdout, stderr string) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		s.baseURL+"/v1.44/containers/"+containerID+"/logs?stdout=1&stderr=1",
		nil)
	if err != nil {
		return "", ""
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	// Docker logs are streamed with header bytes
	// Format: [8]byte{STREAM_TYPE, 0, 0, 0, SIZE1, SIZE2, SIZE3, SIZE4} + payload
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", ""
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	i := 0
	for i < len(data) {
		if i+8 > len(data) {
			break
		}
		// Skip header
		i += 8

		// Find null terminator or next header
		start := i
		for i < len(data) && data[i] != 0 {
			// Check if this looks like a new header
			if i+9 <= len(data) && (data[i] == 1 || data[i] == 2) && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 0 {
				break
			}
			i++
		}

		// Determine stream type from previous header
		if start > 8 {
			streamType := data[start-9]
			payload := data[start:i]
			if streamType == 1 {
				stdoutBuf.Write(payload)
			} else if streamType == 2 {
				stderrBuf.Write(payload)
			}
		}
	}

	return stdoutBuf.String(), stderrBuf.String()
}

// removeContainer deletes a container.
func (s *SandboxClient) removeContainer(ctx context.Context, containerID string) {
	req, _ := http.NewRequestWithContext(ctx, "DELETE",
		s.baseURL+"/v1.44/containers/"+containerID+"?force=true",
		nil)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[sandbox] warning: failed to remove container %s: %v", containerID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		log.Printf("[sandbox] warning: unexpected status removing container %s: %d", containerID, resp.StatusCode)
	}
}

// checkDockerSocket verifies that the Docker socket is accessible.
func checkDockerSocket() error {
	info, err := os.Stat(dockerSocket)
	if err != nil {
		return fmt.Errorf("docker socket not accessible: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("docker socket path is not a socket")
	}
	return nil
}

// Health checks if the sandbox is available.
func (s *SandboxClient) Health() error {
	if err := checkDockerSocket(); err != nil {
		return err
	}
	// Try to ping Docker API
	req, err := http.NewRequest("GET", s.baseURL+"/v1.44/_ping", nil)
	if err != nil {
		return err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("docker api unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker api ping failed: %d", resp.StatusCode)
	}
	return nil
}
