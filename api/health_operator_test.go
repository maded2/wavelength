package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/config"
	"wavelength/internal/llm"
)

// E1-S7: Operator views application health and status

func TestOperatorHealthStatus(t *testing.T) {
	t.Run("health endpoint shows the application is running", func(t *testing.T) {
		app := fiber.New()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer llmServer.Close()

		cfg := &config.Config{
			Server: config.ServerConfig{Port: 3000},
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: llmServer.URL,
				APIKey:   "test-key",
			},
			DataDir: t.TempDir(),
		}

		client := llm.NewClient(cfg)
		app.Get("/health", HealthHandler(client))

		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		var health map[string]interface{}
		if err := json.Unmarshal(body, &health); err != nil {
			t.Fatalf("expected JSON response, got: %s", string(body))
		}

		if health["status"] != "running" {
			t.Errorf("expected application status 'running', got: %v", health["status"])
		}
	})

	t.Run("health status includes whether the LLM backend is reachable", func(t *testing.T) {
		app := fiber.New()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer llmServer.Close()

		cfg := &config.Config{
			Server: config.ServerConfig{Port: 3000},
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: llmServer.URL,
				APIKey:   "test-key",
			},
			DataDir: t.TempDir(),
		}

		client := llm.NewClient(cfg)
		app.Get("/health", HealthHandler(client))

		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		var health map[string]interface{}
		if err := json.Unmarshal(body, &health); err != nil {
			t.Fatalf("expected JSON response, got: %s", string(body))
		}

		llmStatus, ok := health["llm"].(map[string]interface{})
		if !ok {
			t.Fatal("expected 'llm' field in health response")
		}

		status, ok := llmStatus["status"].(string)
		if !ok {
			t.Fatal("expected 'status' field in llm section")
		}

		if status != "available" && status != "unavailable" {
			t.Errorf("expected llm status to be 'available' or 'unavailable', got: %q", status)
		}
	})

	t.Run("health status provides a plain language reason when LLM is unreachable", func(t *testing.T) {
		app := fiber.New()

		cfg := &config.Config{
			Server: config.ServerConfig{Port: 3000},
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: "http://localhost:59999", // non-existent
				APIKey:   "test-key",
			},
			DataDir: t.TempDir(),
		}

		client := llm.NewClient(cfg)
		app.Get("/health", HealthHandler(client))

		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		var health map[string]interface{}
		if err := json.Unmarshal(body, &health); err != nil {
			t.Fatalf("expected JSON response, got: %s", string(body))
		}

		llmStatus, ok := health["llm"].(map[string]interface{})
		if !ok {
			t.Fatal("expected 'llm' field in health response")
		}

		if llmStatus["status"] != "unavailable" {
			t.Errorf("expected llm status 'unavailable', got: %v", llmStatus["status"])
		}

		reason, ok := llmStatus["reason"].(string)
		if !ok || reason == "" {
			t.Error("expected a plain language reason for LLM unavailability")
		}

		// Reason should be in plain language, not raw technical errors
		lowerReason := strings.ToLower(reason)
		hasPlainLanguage := strings.Contains(lowerReason, "cannot connect") ||
			strings.Contains(lowerReason, "unavailable") ||
			strings.Contains(lowerReason, "error")
		if !hasPlainLanguage {
			t.Errorf("expected plain language reason, got: %s", reason)
		}
	})

	t.Run("health status does not expose API keys or credentials", func(t *testing.T) {
		app := fiber.New()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer llmServer.Close()

		cfg := &config.Config{
			Server: config.ServerConfig{Port: 3000},
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: llmServer.URL,
				APIKey:   "sk-proj-abc123supersecretkey456",
			},
			DataDir: t.TempDir(),
		}

		client := llm.NewClient(cfg)
		app.Get("/health", HealthHandler(client))

		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}
		bodyStr := string(body)

		// Verify no API key in response
		if strings.Contains(bodyStr, "sk-proj-abc123supersecretkey456") {
			t.Error("health response should not contain API key")
		}
		if strings.Contains(bodyStr, "supersecretkey") {
			t.Error("health response should not contain any part of the secret key")
		}
	})

	t.Run("health status reflects real-time LLM availability", func(t *testing.T) {
		app := fiber.New()

		// Server that starts unavailable, then becomes available
		available := false
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if available {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		}))
		defer llmServer.Close()

		cfg := &config.Config{
			Server: config.ServerConfig{Port: 3000},
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: llmServer.URL,
				APIKey:   "test-key",
			},
			DataDir: t.TempDir(),
		}

		client := llm.NewClient(cfg)
		app.Get("/health", HealthHandler(client))

		// First check - LLM unavailable
		req1 := httptest.NewRequest("GET", "/health", nil)
		resp1, err := app.Test(req1)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}

		body1, _ := io.ReadAll(resp1.Body)
		resp1.Body.Close()

		var health1 map[string]interface{}
		json.Unmarshal(body1, &health1)

		llmStatus1 := health1["llm"].(map[string]interface{})
		if llmStatus1["status"] != "unavailable" {
			t.Errorf("expected llm status 'unavailable' initially, got: %v", llmStatus1["status"])
		}

		// Make LLM available
		available = true

		// Second check - LLM now available
		req2 := httptest.NewRequest("GET", "/health", nil)
		resp2, err := app.Test(req2)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}

		body2, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()

		var health2 map[string]interface{}
		json.Unmarshal(body2, &health2)

		llmStatus2 := health2["llm"].(map[string]interface{})
		if llmStatus2["status"] != "available" {
			t.Errorf("expected llm status 'available' after recovery, got: %v", llmStatus2["status"])
		}
	})
}
