package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"wavelength/internal/config"
)

// E1-S1: Operator starts application with a configuration file

func TestLoadConfig(t *testing.T) {
	t.Run("launches successfully when a valid JSON configuration file is present", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		validConfig := `{
			"server": {"port": 3000},
			"llm": {
				"provider": "openai",
				"model": "gpt-4",
				"endpoint": "https://api.openai.com/v1",
				"api_key": "test-key"
			},
			"data_dir": "./data"
		}`
		if err := os.WriteFile(cfgPath, []byte(validConfig), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config to be loaded, got nil")
		}
		if cfg.Server.Port != 3000 {
			t.Errorf("expected port 3000, got %d", cfg.Server.Port)
		}
	})

	t.Run("reads all configuration from that single JSON file", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		fullConfig := `{
			"server": {"port": 8080},
			"llm": {
				"provider": "anthropic",
				"model": "claude-3",
				"endpoint": "https://api.anthropic.com/v1",
				"api_key": "sk-ant-123"
			},
			"persona": {
				"system_prompt": "Custom persona prompt here"
			},
			"data_dir": "/var/wavelength/data"
		}`
		if err := os.WriteFile(cfgPath, []byte(fullConfig), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if cfg.Server.Port != 8080 {
			t.Errorf("expected port 8080, got %d", cfg.Server.Port)
		}
		if cfg.LLM.Provider != "anthropic" {
			t.Errorf("expected provider 'anthropic', got '%s'", cfg.LLM.Provider)
		}
		if cfg.LLM.Model != "claude-3" {
			t.Errorf("expected model 'claude-3', got '%s'", cfg.LLM.Model)
		}
		if cfg.LLM.Endpoint != "https://api.anthropic.com/v1" {
			t.Errorf("expected endpoint 'https://api.anthropic.com/v1', got '%s'", cfg.LLM.Endpoint)
		}
		if cfg.LLM.APIKey != "sk-ant-123" {
			t.Errorf("expected api_key 'sk-ant-123', got '%s'", cfg.LLM.APIKey)
		}
		if cfg.Persona.SystemPrompt != "Custom persona prompt here" {
			t.Errorf("expected persona prompt, got '%s'", cfg.Persona.SystemPrompt)
		}
		if cfg.DataDir != "/var/wavelength/data" {
			t.Errorf("expected data_dir '/var/wavelength/data', got '%s'", cfg.DataDir)
		}
	})

	t.Run("provides a clear error message when the configuration file is missing", func(t *testing.T) {
		_, err := config.Load("/nonexistent/path/to/config.json")
		if err == nil {
			t.Fatal("expected error for missing config file, got nil")
		}
		errMsg := err.Error()
		// Error message should be human-readable and explain the problem
		if len(errMsg) == 0 {
			t.Error("expected a descriptive error message, got empty string")
		}
	})

	t.Run("provides a clear error message when the configuration file contains invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		invalidJSON := `{ "server": { "port": 3000, INVALID }`
		if err := os.WriteFile(cfgPath, []byte(invalidJSON), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := config.Load(cfgPath)
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		errMsg := err.Error()
		if len(errMsg) == 0 {
			t.Error("expected a descriptive error message for invalid JSON, got empty string")
		}
	})
}
