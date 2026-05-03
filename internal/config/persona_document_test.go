package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wavelength/internal/config"
)

// E4-S2: AI agent creates an initial requirement document outline

func TestPersonaDocumentOutline(t *testing.T) {
	t.Run("the persona prompt instructs the agent to create a document outline early in the interview", func(t *testing.T) {
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

		// Should mention creating or updating a document
		hasDocument := strings.Contains(lowerPrompt, "document") ||
			strings.Contains(lowerPrompt, "outline") ||
			strings.Contains(lowerPrompt, "requirements document")
		if !hasDocument {
			t.Errorf("expected persona to mention creating a requirements document, got: %s", prompt)
		}
	})

	t.Run("the persona prompt instructs the agent to use markdown format for the document", func(t *testing.T) {
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

		// Should mention markdown
		if !strings.Contains(lowerPrompt, "markdown") {
			t.Errorf("expected persona to mention markdown format, got: %s", prompt)
		}
	})

	t.Run("the persona prompt instructs the agent to tailor the document to the specific requirement", func(t *testing.T) {
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

		// Should mention tailoring or adapting to the domain
		hasTailoring := strings.Contains(lowerPrompt, "adapt") ||
			strings.Contains(lowerPrompt, "domain") ||
			strings.Contains(lowerPrompt, "tailor") ||
			strings.Contains(lowerPrompt, "specific")
		if !hasTailoring {
			t.Errorf("expected persona to instruct tailoring to the specific requirement, got: %s", prompt)
		}
	})
}
