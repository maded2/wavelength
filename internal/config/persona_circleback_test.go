package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wavelength/internal/config"
)

// E3-S5: AI agent circles back to re-evaluate earlier conclusions

func TestPersonaCircleBack(t *testing.T) {
	t.Run("the persona prompt instructs the agent to revisit earlier conclusions", func(t *testing.T) {
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

		// Should mention circling back or re-evaluating
		hasCircleBack := strings.Contains(lowerPrompt, "circle back") ||
			strings.Contains(lowerPrompt, "revisit") ||
			strings.Contains(lowerPrompt, "re-evaluate") ||
			strings.Contains(lowerPrompt, "re-eval")
		if !hasCircleBack {
			t.Errorf("expected persona to instruct circling back, got: %s", prompt)
		}
	})

	t.Run("the persona prompt instructs the agent to reference specific earlier conclusions when circling back", func(t *testing.T) {
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

		// Should mention earlier conclusions or new information
		hasReference := strings.Contains(lowerPrompt, "earlier") ||
			strings.Contains(lowerPrompt, "conclusion") ||
			strings.Contains(lowerPrompt, "new information")
		if !hasReference {
			t.Errorf("expected persona to instruct referencing earlier conclusions, got: %s", prompt)
		}
	})

	t.Run("the persona prompt instructs the agent not to circle back excessively", func(t *testing.T) {
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

		// Should instruct moderation in circling back
		hasModeration := strings.Contains(lowerPrompt, "judgment") ||
			strings.Contains(lowerPrompt, "natural") ||
			strings.Contains(lowerPrompt, "mechanical") ||
			strings.Contains(lowerPrompt, "excessive")
		if !hasModeration {
			t.Errorf("expected persona to instruct moderate circling back, got: %s", prompt)
		}
	})
}
