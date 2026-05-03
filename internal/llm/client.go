package llm

import (
	"context"
	"fmt"
	"net/http"
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

// Endpoint returns the configured LLM endpoint URL.
func (c *Client) Endpoint() string {
	return c.cfg.LLM.Endpoint
}

// APIKey returns the configured LLM API key.
func (c *Client) APIKey() string {
	return c.cfg.LLM.APIKey
}

// PersonaPrompt returns the configured persona system prompt.
func (c *Client) PersonaPrompt() string {
	return c.cfg.GetPersonaPrompt()
}
