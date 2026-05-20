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

		if !strings.Contains(lowerPrompt, "business analyst") {
			t.Errorf("expected default persona to mention 'business analyst', got: %s", prompt)
		}
	})

	t.Run("changing the persona prompt in config causes the agent to adopt the new persona", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")

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

		if cfg1.GetPersonaPrompt() == cfg2.GetPersonaPrompt() {
			t.Error("expected different persona prompts after config change")
		}
		if cfg2.GetPersonaPrompt() != "You are a technical architect focused on system design." {
			t.Errorf("expected new persona prompt, got: %q", cfg2.GetPersonaPrompt())
		}
	})
}

// E3-S3: AI agent systematically probes standard requirement categories
// E3-S4: AI agent identifies gaps, ambiguities, and contradictions
// E3-S5: AI agent circles back to re-evaluate earlier conclusions
// E4-S2: AI agent creates an initial requirement document outline
// E4-S3: AI agent updates the requirement document as the interview progresses

func TestPersonaPromptContent(t *testing.T) {
	// Load the default persona prompt once
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{
		"server": {"port": 3000},
		"llm": {
			"provider": "openai",
			"model": "gpt-4",
			"endpoint": "https://api.openai.com/v1",
			"api_key": "test-key"
		},
		"data_dir": "./data"
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	prompt := cfg.GetPersonaPrompt()
	lowerPrompt := strings.ToLower(prompt)

	// Each keyword is its own test case for granular failure diagnosis
	tests := []struct {
		name   string
		keyword string
		category string
	}{
		// Key requirement dimensions
		{"covers persona", "persona", "key requirement categories"},
		{"covers workflow", "workflow", "key requirement categories"},
		{"covers business rule", "business rule", "key requirement categories"},
		{"covers constraint", "constraint", "key requirement categories"},
		{"covers edge case", "edge case", "key requirement categories"},
		{"covers non-functional", "non-functional", "key requirement categories"},
		{"covers dependencies", "depend", "key requirement categories"},
		{"covers assumptions", "assumption", "key requirement categories"},
		// Progressive/conversational approach
		{"uses progressive questioning", "progressive", "progressive approach"},
		{"uses conversational style", "conversational", "progressive approach"},
		{"adapts to context", "adapt", "progressive approach"},
		{"maintains natural tone", "natural", "progressive approach"},
		// Gap detection
		{"detects gaps", "gap", "gap detection"},
		{"detects ambiguities", "ambiguit", "gap detection"},
		{"detects vague answers", "vague", "gap detection"},
		// Clarifying undefined terms
		{"asks for clarification", "clarif", "clarifying terms"},
		{"asks for definitions", "defin", "clarifying terms"},
		{"asks for specifics", "specific", "clarifying terms"},
		// Contradiction detection
		{"surfaces contradictions", "contradict", "contradiction detection"},
		// Circling back
		{"circles back to earlier points", "circle back", "circling back"},
		{"re-evaluates conclusions", "re-evaluate", "circling back"},
		{"references earlier conclusions", "earlier", "circling back"},
		{"references conclusions", "conclusion", "circling back"},
		{"considers new information", "new information", "circling back"},
		// Document creation
		{"creates requirements document", "document", "document creation"},
		{"creates requirements document (explicit)", "requirements document", "document creation"},
		{"uses markdown format", "markdown", "document creation"},
		{"updates document", "update", "document creation"},
		{"maintains structured format", "structured", "document creation"},
		{"maintains consistent format", "format", "document creation"},
		// Tailoring
		{"adapts to domain", "domain", "tailoring"},
		// Moderate circling back
		{"adaptive circling back", "adaptive", "moderate circling back"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(lowerPrompt, tc.keyword) {
				t.Errorf("expected persona to mention %q (%s), got: %s", tc.keyword, tc.category, prompt)
			}
		})
	}
}

// E1-S4: persona prompt can be configured to instruct the agent to avoid implementation advice
func TestPersonaCustomization(t *testing.T) {
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
}
