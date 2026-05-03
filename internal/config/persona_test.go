package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wavelength/internal/config"
)

// E1-S4: Operator configures the AI agent's business analyst persona

func TestPersonaConfig(t *testing.T) {
	t.Run("configuration file includes a section for the AI agent persona prompt", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		cfgJSON := `{
			"server": {"port": 3000},
			"llm": {
				"provider": "openai",
				"model": "gpt-4",
				"endpoint": "https://api.openai.com/v1",
				"api_key": "test-key"
			},
			"persona": {
				"system_prompt": "You are a business analyst conducting a requirements gathering session."
			},
			"data_dir": "./data"
		}`
		if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if cfg.Persona.SystemPrompt != "You are a business analyst conducting a requirements gathering session." {
			t.Errorf("expected persona prompt to be loaded, got: %q", cfg.Persona.SystemPrompt)
		}
	})

	t.Run("when no persona prompt is configured the application uses a sensible default", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		cfgJSON := `{
			"server": {"port": 3000},
			"llm": {
				"provider": "openai",
				"model": "gpt-4",
				"endpoint": "https://api.openai.com/v1",
				"api_key": "test-key"
			},
			"data_dir": "./data"
		}`
		if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// GetPersonaPrompt should return a default when none is configured
		prompt := cfg.GetPersonaPrompt()
		if prompt == "" {
			t.Error("expected default persona prompt, got empty string")
		}
	})

	t.Run("the default persona prompt positions the agent as a business analyst", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		cfgJSON := `{
			"server": {"port": 3000},
			"llm": {
				"provider": "openai",
				"model": "gpt-4",
				"endpoint": "https://api.openai.com/v1",
				"api_key": "test-key"
			},
			"data_dir": "./data"
		}`
		if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		prompt := cfg.GetPersonaPrompt()
		lowerPrompt := strings.ToLower(prompt)

		// The default should mention business analyst role
		if !strings.Contains(lowerPrompt, "business analyst") {
			t.Errorf("expected default persona to mention 'business analyst', got: %s", prompt)
		}
	})

	t.Run("the persona prompt constrains the agent to focus on requirements elicitation", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		cfgJSON := `{
			"server": {"port": 3000},
			"llm": {
				"provider": "openai",
				"model": "gpt-4",
				"endpoint": "https://api.openai.com/v1",
				"api_key": "test-key"
			},
			"data_dir": "./data"
		}`
		if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		prompt := cfg.GetPersonaPrompt()
		lowerPrompt := strings.ToLower(prompt)

		// The default should mention requirements or elicitation
		hasRequirements := strings.Contains(lowerPrompt, "requirement") ||
			strings.Contains(lowerPrompt, "elicitation")
		if !hasRequirements {
			t.Errorf("expected default persona to mention requirements or elicitation, got: %s", prompt)
		}
	})

	t.Run("the persona prompt can be configured to instruct the agent to avoid implementation advice", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		cfgJSON := `{
			"server": {"port": 3000},
			"llm": {
				"provider": "openai",
				"model": "gpt-4",
				"endpoint": "https://api.openai.com/v1",
				"api_key": "test-key"
			},
			"persona": {
				"system_prompt": "You are a business analyst. Focus on understanding requirements. Do not provide implementation or architectural advice."
			},
			"data_dir": "./data"
		}`
		if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		prompt := cfg.GetPersonaPrompt()
		if !strings.Contains(prompt, "implementation") {
			t.Errorf("expected configured persona to contain implementation guidance, got: %s", prompt)
		}
	})

	t.Run("changing the persona prompt in config causes the agent to adopt the new persona", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")

		// First config with one persona
		cfgJSON1 := `{
			"server": {"port": 3000},
			"llm": {
				"provider": "openai",
				"model": "gpt-4",
				"endpoint": "https://api.openai.com/v1",
				"api_key": "test-key"
			},
			"persona": {
				"system_prompt": "You are a business analyst focused on requirements."
			},
			"data_dir": "./data"
		}`
		if err := os.WriteFile(cfgPath, []byte(cfgJSON1), 0644); err != nil {
			t.Fatal(err)
		}

		cfg1, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Second config with different persona
		cfgJSON2 := `{
			"server": {"port": 3000},
			"llm": {
				"provider": "openai",
				"model": "gpt-4",
				"endpoint": "https://api.openai.com/v1",
				"api_key": "test-key"
			},
			"persona": {
				"system_prompt": "You are a technical architect focused on system design."
			},
			"data_dir": "./data"
		}`
		if err := os.WriteFile(cfgPath, []byte(cfgJSON2), 0644); err != nil {
			t.Fatal(err)
		}

		cfg2, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify the persona changed
		if cfg1.GetPersonaPrompt() == cfg2.GetPersonaPrompt() {
			t.Error("expected different persona prompts after config change")
		}
		if cfg2.GetPersonaPrompt() != "You are a technical architect focused on system design." {
			t.Errorf("expected new persona prompt, got: %q", cfg2.GetPersonaPrompt())
		}
	})
}
