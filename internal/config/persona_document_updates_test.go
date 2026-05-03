package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wavelength/internal/config"
)

// E4-S3: AI agent updates the requirement document as the interview progresses

func TestPersonaDocumentUpdates(t *testing.T) {
	t.Run("the persona prompt instructs the agent to update the document as new information is gathered", func(t *testing.T) {
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

		// Should mention updating the document
		hasUpdate := strings.Contains(lowerPrompt, "update") ||
			strings.Contains(lowerPrompt, "evolve") ||
			strings.Contains(lowerPrompt, "progress")
		if !hasUpdate {
			t.Errorf("expected persona to instruct document updates, got: %s", prompt)
		}
	})

	t.Run("the persona prompt instructs the agent to add new sections and refine existing content", func(t *testing.T) {
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

		// Should mention adding sections or refining
		hasRefine := strings.Contains(lowerPrompt, "section") ||
			strings.Contains(lowerPrompt, "refine") ||
			strings.Contains(lowerPrompt, "detail") ||
			strings.Contains(lowerPrompt, "structured")
		if !hasRefine {
			t.Errorf("expected persona to instruct adding sections and refining content, got: %s", prompt)
		}
	})

	t.Run("the persona prompt instructs the agent to maintain consistent structure as the document grows", func(t *testing.T) {
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

		// Should mention structure or formatting
		hasStructure := strings.Contains(lowerPrompt, "structure") ||
			strings.Contains(lowerPrompt, "format") ||
			strings.Contains(lowerPrompt, "organized")
		if !hasStructure {
			t.Errorf("expected persona to instruct maintaining consistent structure, got: %s", prompt)
		}
	})
}
