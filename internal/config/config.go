package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config holds the complete application configuration loaded from a single JSON file.
type Config struct {
	Server  ServerConfig  `json:"server"`
	LLM     LLMConfig     `json:"llm"`
	Persona PersonaConfig `json:"persona"`
	DataDir string        `json:"data_dir"`
}

// ServerConfig holds server-related settings.
type ServerConfig struct {
	Port int `json:"port"`
}

// LLMConfig holds LLM backend configuration.
type LLMConfig struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Endpoint    string  `json:"endpoint"`
	APIKey      string  `json:"api_key"`
	Temperature float64 `json:"temperature"`
	Timeout     int     `json:"timeout"`       // HTTP request timeout in seconds (default 60)
	Path        string  `json:"path"`          // API path appended to endpoint (default "/chat/completions")
}

// PersonaConfig holds the AI agent persona configuration.
type PersonaConfig struct {
	SystemPrompt string `json:"system_prompt"`
}

// Load reads and parses a JSON configuration file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("configuration file not found at %q: %w. Please provide a valid JSON configuration file at this path", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("configuration file at %q contains invalid JSON: %w", path, err)
	}

	return &cfg, nil
}

// Validate checks that all required configuration fields are present and valid.
func (c *Config) Validate() error {
	var errs []string

	if c.Server.Port <= 0 {
		errs = append(errs, "server.port must be a positive integer")
	}

	if c.LLM.Provider == "" {
		errs = append(errs, "llm.provider is required (e.g., 'openai', 'anthropic')")
	}
	if c.LLM.Model == "" {
		errs = append(errs, "llm.model is required (e.g., 'gpt-4', 'claude-3')")
	}
	if c.LLM.Endpoint == "" {
		errs = append(errs, "llm.endpoint is required (e.g., 'https://api.openai.com/v1')")
	}
	if c.LLM.APIKey == "" {
		errs = append(errs, "llm.api_key is required")
	}

	if c.DataDir == "" {
		errs = append(errs, "data_dir is required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// RedactedLLMConfig returns a copy of the LLM config with sensitive fields removed.
func (c *Config) RedactedLLMConfig() map[string]interface{} {
	return map[string]interface{}{
		"provider":  c.LLM.Provider,
		"model":     c.LLM.Model,
		"endpoint":  c.LLM.Endpoint,
		"api_key":   "***redacted***",
		"temperature": c.LLM.Temperature,
	}
}

// DefaultPersonaPrompt is the default system prompt that positions the AI agent
// as a business analyst focused on requirements elicitation.
const DefaultPersonaPrompt = `You are a business analyst working for the IT department conducting a requirements gathering session with stakeholders and product owners.

Your role is to conduct an interview-style conversation to expand and drill into the details of the requirements needed to design a system. You will also create and maintain a living requirements document in markdown format that captures all elicited requirements.

Guidelines:
- Ask targeted questions to elicit detailed requirements from the stakeholder
- Cover all important dimensions progressively: user personas, functional workflows, business rules, constraints, edge cases, non-functional requirements, dependencies, and assumptions
- Do not ask about all categories at once; explore them progressively following conversational threads naturally
- If answers make a category irrelevant, recognize this and skip it rather than forcing questions about it
- Probe for gaps, ambiguities, and contradictions in the stakeholder's answers
- When the stakeholder uses vague or undefined terms, ask for specific clarification and measurable details
- Periodically circle back to re-evaluate earlier conclusions as new information emerges
- Create and update a structured requirements document in markdown format as the interview progresses
- Tailor the document structure and content to the specific domain and requirements being discussed
- Focus on understanding what the system should do, not how to implement it
- Do NOT provide implementation details, architectural advice, or technical solutions
- Keep the conversation natural and adaptive to the domain being discussed

Document Updates:
Whenever you update or create the living requirements document, include the complete document content wrapped in --- delimiters. Format:

---
<complete markdown document content here>
---

The content between the delimiters will be extracted and saved as the topic's requirement document. Everything outside the delimiters is your conversational response to the stakeholder.`

// GetPersonaPrompt returns the configured persona prompt, or the default if none is set.
func (c *Config) GetPersonaPrompt() string {
	if c.Persona.SystemPrompt != "" {
		return c.Persona.SystemPrompt
	}
	return DefaultPersonaPrompt
}

var (
	ErrMissingProvider    = errors.New("llm.provider is required")
	ErrMissingModel       = errors.New("llm.model is required")
	ErrMissingEndpoint    = errors.New("llm.endpoint is required")
	ErrMissingAPIKey      = errors.New("llm.api_key is required")
	ErrMissingDataDir     = errors.New("data_dir is required")
	ErrInvalidServerPort  = errors.New("server.port must be a positive integer")
)
