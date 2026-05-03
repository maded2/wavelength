package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wavelength/internal/config"
)

// E3-S4: AI agent identifies gaps, ambiguities, and contradictions

func TestPersonaGapDetection(t *testing.T) {
	t.Run("the persona prompt instructs the agent to ask for clarification on vague answers", func(t *testing.T) {
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

		// Should mention gaps, ambiguities, or vague answers
		hasGapDetection := strings.Contains(lowerPrompt, "gap") ||
			strings.Contains(lowerPrompt, "ambiguit") ||
			strings.Contains(lowerPrompt, "vague")
		if !hasGapDetection {
			t.Errorf("expected persona to instruct gap detection, got: %s", prompt)
		}
	})

	t.Run("the persona prompt instructs the agent to ask for definitions of undefined terms", func(t *testing.T) {
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

		// Should mention clarifying or defining terms
		hasDefineTerms := strings.Contains(lowerPrompt, "clarif") ||
			strings.Contains(lowerPrompt, "defin") ||
			strings.Contains(lowerPrompt, "specific")
		if !hasDefineTerms {
			t.Errorf("expected persona to instruct clarifying undefined terms, got: %s", prompt)
		}
	})

	t.Run("the persona prompt instructs the agent to surface contradictions", func(t *testing.T) {
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

		// Should mention contradictions
		if !strings.Contains(lowerPrompt, "contradict") {
			t.Errorf("expected persona to instruct contradiction detection, got: %s", prompt)
		}
	})

	t.Run("the persona prompt instructs constructive non-accusatory probing", func(t *testing.T) {
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

		// Should instruct constructive/helpful approach
		hasConstructive := strings.Contains(lowerPrompt, "helpful") ||
			strings.Contains(lowerPrompt, "constructive") ||
			strings.Contains(lowerPrompt, "professional") ||
			strings.Contains(lowerPrompt, "natural")
		if !hasConstructive {
			t.Errorf("expected persona to instruct constructive probing style, got: %s", prompt)
		}
	})
}
