package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"wavelength/internal/config"
)

// Client represents an LLM client that can check connectivity to the configured endpoint.
type Client struct {
	cfg *config.Config
}

// NewClient creates a new LLM client with the given configuration.
func NewClient(cfg *config.Config) *Client {
	return &Client{cfg: cfg}
}

// CheckConnectivity performs a basic connectivity check to the configured LLM endpoint.
// Returns nil if the endpoint is reachable, or an error with a descriptive message.
func (c *Client) CheckConnectivity(ctx context.Context) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.cfg.LLM.Endpoint, nil)
	if err != nil {
		return fmt.Errorf("cannot connect to LLM service: invalid endpoint URL %q", c.cfg.LLM.Endpoint)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.LLM.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot connect to LLM service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("cannot connect to LLM service: authentication failed")
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("cannot connect to LLM service: server returned status %d", resp.StatusCode)
	}

	return nil
}

// Model returns the configured LLM model name.
func (c *Client) Model() string {
	return c.cfg.LLM.Model
}

// Endpoint returns the configured LLM endpoint base URL.
func (c *Client) Endpoint() string {
	return c.cfg.LLM.Endpoint
}

// APIPath returns the configured API path (default "/chat/completions").
func (c *Client) APIPath() string {
	if c.cfg.LLM.Path != "" {
		return c.cfg.LLM.Path
	}
	return "/chat/completions"
}

// APIURL returns the full URL for chat completions (endpoint + path).
func (c *Client) APIURL() string {
	return c.cfg.LLM.Endpoint + c.APIPath()
}

// Timeout returns the configured HTTP timeout in seconds (default 60).
func (c *Client) Timeout() int {
	if c.cfg.LLM.Timeout > 0 {
		return c.cfg.LLM.Timeout
	}
	return 60
}

// APIKey returns the configured LLM API key.
func (c *Client) APIKey() string {
	return c.cfg.LLM.APIKey
}

// PersonaPrompt returns the configured persona system prompt.
func (c *Client) PersonaPrompt() string {
	return c.cfg.GetPersonaPrompt()
}

// StreamResponse sends a chat completion request with streaming enabled and writes
// SSE events to the provided writer. Each token chunk is emitted as a JSON event.
// The caller is responsible for flushing the writer after each event.
func (c *Client) StreamResponse(ctx context.Context, w io.Writer, systemPrompt string, messages []Message) error {
	msgPayload := make([]map[string]string, 0, len(messages)+1)
	msgPayload = append(msgPayload, map[string]string{"role": "system", "content": systemPrompt})
	for _, m := range messages {
		msgPayload = append(msgPayload, map[string]string{"role": m.Role, "content": m.Content})
	}

	payload := map[string]interface{}{
		"model":       c.cfg.LLM.Model,
		"stream":      true,
		"temperature": c.cfg.LLM.Temperature,
		"max_tokens":  4096,
		"messages":    msgPayload,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to prepare LLM request: %v", err)
	}

	timeout := time.Duration(c.cfg.LLM.Timeout) * time.Second
	req, err := http.NewRequestWithContext(ctx, "POST", c.APIURL(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cannot connect to LLM service: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.LLM.APIKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot connect to LLM service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("LLM service error: status %d", resp.StatusCode)
	}

	// Parse Server-Sent Events stream
	scanner := bufio.NewScanner(resp.Body)
	// Increase buffer size for large tokens
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			// Emit done event
			event := map[string]interface{}{
				"type": "done",
			}
			json.NewEncoder(w).Encode(event)
			return nil
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		choices, ok := chunk["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			continue
		}

		firstChoice, ok := choices[0].(map[string]interface{})
		if !ok {
			continue
		}

		delta, ok := firstChoice["delta"].(map[string]interface{})
		if !ok {
			continue
		}

		content, ok := delta["content"].(string)
		if !ok || content == "" {
			continue
		}

		// Emit token event
		event := map[string]interface{}{
			"type":    "token",
			"content": content,
		}
		if err := json.NewEncoder(w).Encode(event); err != nil {
			return fmt.Errorf("failed to write stream event: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read error: %v", err)
	}

	// Emit done if we reach here without [DONE]
	event := map[string]interface{}{
		"type": "done",
	}
	json.NewEncoder(w).Encode(event)
	return nil
}

// Message represents a chat message for the LLM API.
type Message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}
