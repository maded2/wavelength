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
	tests := []struct {
		name          string
		contains      []string
		notContains   []string
		description   string
	}{
		{
			name: "instructs the agent to cover key requirement dimensions",
			contains: []string{
				"persona", "workflow", "business rule", "constraint",
				"edge case", "non-functional", "depend", "assumption",
			},
			description: "key requirement categories",
		},
		{
			name: "instructs progressive questioning not all at once",
			contains: []string{
				"progressive", "conversational", "adapt", "natural",
			},
			description: "progressive/conversational approach",
		},
		{
			name: "instructs the agent to skip irrelevant categories",
			contains: []string{
				"irrelevant", "skip",
			},
			description: "handling irrelevant categories",
		},
		{
			name: "instructs the agent to ask for clarification on vague answers",
			contains: []string{
				"gap", "ambiguit", "vague",
			},
			description: "gap detection",
		},
		{
			name: "instructs the agent to ask for definitions of undefined terms",
			contains: []string{
				"clarif", "defin", "specific",
			},
			description: "clarifying undefined terms",
		},
		{
			name: "instructs the agent to surface contradictions",
			contains: []string{
				"contradict",
			},
			description: "contradiction detection",
		},
		{
			name: "instructs constructive non-accusatory probing",
			contains: []string{
				"natural",
			},
			description: "constructive probing style",
		},
		{
			name: "instructs the agent to revisit earlier conclusions",
			contains: []string{
				"circle back", "re-evaluate",
			},
			description: "circling back",
		},
		{
			name: "instructs the agent to reference specific earlier conclusions",
			contains: []string{
				"earlier", "conclusion", "new information",
			},
			description: "referencing earlier conclusions",
		},
		{
			name: "instructs the agent not to circle back excessively",
			contains: []string{
				"natural", "adaptive",
			},
			description: "moderate circling back",
		},
		{
			name: "instructs the agent to create a document outline early",
			contains: []string{
				"document", "requirements document",
			},
			description: "creating a requirements document",
		},
		{
			name: "instructs the agent to use markdown format for the document",
			contains: []string{
				"markdown",
			},
			description: "markdown format",
		},
		{
			name: "instructs the agent to tailor the document to the specific requirement",
			contains: []string{
				"adapt", "domain", "tailor", "specific",
			},
			description: "tailoring to the specific requirement",
		},
		{
			name: "instructs the agent to update the document as new information is gathered",
			contains: []string{
				"update",
			},
			description: "document updates",
		},
		{
			name: "instructs the agent to add new sections and refine existing content",
			contains: []string{
				"structured",
			},
			description: "adding sections and refining content",
		},
		{
			name: "instructs the agent to maintain consistent structure as the document grows",
			contains: []string{
				"structured", "format",
			},
			description: "maintaining consistent structure",
		},
	}

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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, want := range tc.contains {
				if !strings.Contains(lowerPrompt, want) {
					t.Errorf("expected persona to mention %q (%s), got: %s", want, tc.description, prompt)
				}
			}
			for _, want := range tc.notContains {
				if strings.Contains(lowerPrompt, want) {
					t.Errorf("expected persona to NOT mention %q (%s), got: %s", want, tc.description, prompt)
				}
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
