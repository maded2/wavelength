package config

import (
	"encoding/json"
	"fmt"
	"os"
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
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	Endpoint string  `json:"endpoint"`
	APIKey   string  `json:"api_key"`
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
