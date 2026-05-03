package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wavelength/internal/config"
)

// E3-S3: AI agent systematically probes standard requirement categories

func TestPersonaProbesRequirementCategories(t *testing.T) {
	t.Run("the default persona prompt instructs the agent to cover key requirement dimensions", func(t *testing.T) {
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

		// The persona should mention key requirement categories
		categories := []string{
			"persona",
			"workflow",
			"business rule",
			"constraint",
			"edge case",
			"non-functional",
			"depend",
			"assumption",
		}

		for _, category := range categories {
			if !strings.Contains(lowerPrompt, category) {
				t.Errorf("expected persona prompt to mention %q requirement category, got: %s", category, prompt)
			}
		}
	})

	t.Run("the persona prompt instructs progressive questioning not all at once", func(t *testing.T) {
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

		// Should instruct progressive/conversational approach
		hasProgressive := strings.Contains(lowerPrompt, "progressive") ||
			strings.Contains(lowerPrompt, "conversational") ||
			strings.Contains(lowerPrompt, "adapt") ||
			strings.Contains(lowerPrompt, "natural")
		if !hasProgressive {
			t.Errorf("expected persona to instruct progressive questioning, got: %s", prompt)
		}
	})

	t.Run("the persona prompt instructs the agent to skip irrelevant categories", func(t *testing.T) {
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

		// Should instruct to skip irrelevant categories
		hasSkipIrrelevant := strings.Contains(lowerPrompt, "irrelevant") ||
			strings.Contains(lowerPrompt, "skip") ||
			strings.Contains(lowerPrompt, "force")
		if !hasSkipIrrelevant {
			t.Errorf("expected persona to mention handling irrelevant categories, got: %s", prompt)
		}
	})
}
