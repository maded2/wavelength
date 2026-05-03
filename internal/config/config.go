package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config holds the complete application configuration loaded from a single JSON file.
type Config struct {
	Server  ServerConfig  `json:"server"`
	LLM     LLMConfig     `json:"llm"`
	Persona PersonaConfig `json:"persona"`
	DataDir string        `json:"data_dir"`
}

// ServerConfig holds server-related settings.
type ServerConfig struct {
	Port int `json:"port"`
}

// LLMConfig holds LLM backend configuration.
type LLMConfig struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Endpoint    string  `json:"endpoint"`
	APIKey      string  `json:"api_key"`
	Temperature float64 `json:"temperature"`
}

// PersonaConfig holds the AI agent persona configuration.
type PersonaConfig struct {
	SystemPrompt string `json:"system_prompt"`
}

// Load reads and parses a JSON configuration file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("configuration file not found at %q: %w. Please provide a valid JSON configuration file at this path", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("configuration file at %q contains invalid JSON: %w", path, err)
	}

	return &cfg, nil
}

// Validate checks that all required configuration fields are present and valid.
func (c *Config) Validate() error {
	var errs []string

	if c.Server.Port <= 0 {
		errs = append(errs, "server.port must be a positive integer")
	}

	if c.LLM.Provider == "" {
		errs = append(errs, "llm.provider is required (e.g., 'openai', 'anthropic')")
	}
	if c.LLM.Model == "" {
		errs = append(errs, "llm.model is required (e.g., 'gpt-4', 'claude-3')")
	}
	if c.LLM.Endpoint == "" {
		errs = append(errs, "llm.endpoint is required (e.g., 'https://api.openai.com/v1')")
	}
	if c.LLM.APIKey == "" {
		errs = append(errs, "llm.api_key is required")
	}

	if c.DataDir == "" {
		errs = append(errs, "data_dir is required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// RedactedLLMConfig returns a copy of the LLM config with sensitive fields removed.
func (c *Config) RedactedLLMConfig() map[string]interface{} {
	return map[string]interface{}{
		"provider":  c.LLM.Provider,
		"model":     c.LLM.Model,
		"endpoint":  c.LLM.Endpoint,
		"api_key":   "***redacted***",
		"temperature": c.LLM.Temperature,
	}
}

var (
	ErrMissingProvider    = errors.New("llm.provider is required")
	ErrMissingModel       = errors.New("llm.model is required")
	ErrMissingEndpoint    = errors.New("llm.endpoint is required")
	ErrMissingAPIKey      = errors.New("llm.api_key is required")
	ErrMissingDataDir     = errors.New("data_dir is required")
	ErrInvalidServerPort  = errors.New("server.port must be a positive integer")
)
