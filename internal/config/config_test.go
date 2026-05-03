package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// E1-S2: Operator configures the LLM backend via the configuration file

func TestLLMConfigValidation(t *testing.T) {
	tests := []struct {
		name          string
		configJSON    string
		expectError   bool
		errorContains []string
	}{
		{
			name: "valid LLM configuration passes validation",
			configJSON: `{
				"server": {"port": 3000},
				"llm": {
					"provider": "openai",
					"model": "gpt-4",
					"endpoint": "https://api.openai.com/v1",
					"api_key": "test-key"
				},
				"data_dir": "./data"
			}`,
			expectError: false,
		},
		{
			name: "missing LLM provider returns error",
			configJSON: `{
				"server": {"port": 3000},
				"llm": {
					"model": "gpt-4",
					"endpoint": "https://api.openai.com/v1",
					"api_key": "test-key"
				},
				"data_dir": "./data"
			}`,
			expectError:   true,
			errorContains: []string{"provider"},
		},
		{
			name: "missing LLM model returns error",
			configJSON: `{
				"server": {"port": 3000},
				"llm": {
					"provider": "openai",
					"endpoint": "https://api.openai.com/v1",
					"api_key": "test-key"
				},
				"data_dir": "./data"
			}`,
			expectError:   true,
			errorContains: []string{"model"},
		},
		{
			name: "missing LLM endpoint returns error",
			configJSON: `{
				"server": {"port": 3000},
				"llm": {
					"provider": "openai",
					"model": "gpt-4",
					"api_key": "test-key"
				},
				"data_dir": "./data"
			}`,
			expectError:   true,
			errorContains: []string{"endpoint"},
		},
		{
			name: "missing LLM api_key returns error",
			configJSON: `{
				"server": {"port": 3000},
				"llm": {
					"provider": "openai",
					"model": "gpt-4",
					"endpoint": "https://api.openai.com/v1"
				},
				"data_dir": "./data"
			}`,
			expectError:   true,
			errorContains: []string{"api_key"},
		},
		{
			name: "missing entire LLM section returns errors for all required fields",
			configJSON: `{
				"server": {"port": 3000},
				"data_dir": "./data"
			}`,
			expectError:   true,
			errorContains: []string{"provider", "model", "endpoint", "api_key"},
		},
		{
			name: "invalid server port returns error",
			configJSON: `{
				"server": {"port": 0},
				"llm": {
					"provider": "openai",
					"model": "gpt-4",
					"endpoint": "https://api.openai.com/v1",
					"api_key": "test-key"
				},
				"data_dir": "./data"
			}`,
			expectError:   true,
			errorContains: []string{"port"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "config.json")
			if err := os.WriteFile(cfgPath, []byte(tc.configJSON), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				t.Fatalf("failed to load config: %v", err)
			}

			err = cfg.Validate()
			if tc.expectError {
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}
				errMsg := err.Error()
				for _, contains := range tc.errorContains {
					if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(contains)) {
						t.Errorf("expected error to contain %q, got: %s", contains, errMsg)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("expected no validation error, got: %v", err)
				}
			}
		})
	}
}

func TestLLMConfigSwappable(t *testing.T) {
	// Verify that different LLM configurations can be loaded without hardcoded values
	providers := []struct {
		provider string
		model    string
		endpoint string
	}{
		{"openai", "gpt-4", "https://api.openai.com/v1"},
		{"anthropic", "claude-3", "https://api.anthropic.com/v1"},
		{"ollama", "llama3", "http://localhost:11434"},
		{"custom", "custom-model", "https://custom-llm.example.com/api"},
	}

	for _, p := range providers {
		t.Run("configures "+p.provider, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "config.json")
			cfgJSON := map[string]interface{}{
				"server": map[string]interface{}{"port": 3000},
				"llm": map[string]interface{}{
					"provider": p.provider,
					"model":    p.model,
					"endpoint": p.endpoint,
					"api_key":  "test-key",
				},
				"data_dir": dir,
			}
			data, _ := json.Marshal(cfgJSON)
			if err := os.WriteFile(cfgPath, data, 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				t.Fatalf("failed to load config for %s: %v", p.provider, err)
			}

			if err := cfg.Validate(); err != nil {
				t.Fatalf("validation failed for %s: %v", p.provider, err)
			}

			if cfg.LLM.Provider != p.provider {
				t.Errorf("expected provider %s, got %s", p.provider, cfg.LLM.Provider)
			}
			if cfg.LLM.Model != p.model {
				t.Errorf("expected model %s, got %s", p.model, cfg.LLM.Model)
			}
			if cfg.LLM.Endpoint != p.endpoint {
				t.Errorf("expected endpoint %s, got %s", p.endpoint, cfg.LLM.Endpoint)
			}
		})
	}
}

func TestLLMCredentialsNotExposed(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfgJSON := `{
		"server": {"port": 3000},
		"llm": {
			"provider": "openai",
			"model": "gpt-4",
			"endpoint": "https://api.openai.com/v1",
			"api_key": "super-secret-key-12345"
		},
		"data_dir": "./data"
	}`
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify RedactedLLMConfig does not expose the API key
	redacted := cfg.RedactedLLMConfig()
	if apiKey, ok := redacted["api_key"].(string); ok {
		if apiKey != "***redacted***" {
			t.Errorf("expected redacted API key, got: %s", apiKey)
		}
	} else {
		t.Error("expected api_key field in redacted config")
	}

	// Verify other fields are still present
	if redacted["provider"] != "openai" {
		t.Errorf("expected provider 'openai', got %v", redacted["provider"])
	}
	if redacted["model"] != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %v", redacted["model"])
	}
}
