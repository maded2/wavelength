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

// E1-S5: System verifies LLM connectivity at startup
// E1-S7: Operator views application health and status

func TestHealthEndpoint(t *testing.T) {
	t.Run("health endpoint returns application status", func(t *testing.T) {
		app := fiber.New()

		// Create a test LLM server that responds successfully
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
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

		// Check that LLM status is present and available
		llmStatus, ok := health["llm"].(map[string]interface{})
		if !ok {
			t.Fatal("expected 'llm' field in health response")
		}
		if llmStatus["status"] != "available" {
			t.Errorf("expected llm status 'available', got: %v", llmStatus["status"])
		}
	})

	t.Run("health endpoint reports LLM as unavailable when endpoint is unreachable", func(t *testing.T) {
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

		llmStatus, ok := health["llm"].(map[string]interface{})
		if !ok {
			t.Fatal("expected 'llm' field in health response")
		}
		if llmStatus["status"] != "unavailable" {
			t.Errorf("expected llm status 'unavailable', got: %v", llmStatus["status"])
		}

		// Should include a reason
		reason, ok := llmStatus["reason"].(string)
		if !ok || reason == "" {
			t.Error("expected a reason for LLM unavailability")
		}
	})

	t.Run("health endpoint does not expose sensitive data like API keys", func(t *testing.T) {
		app := fiber.New()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer llmServer.Close()

		cfg := &config.Config{
			Server: config.ServerConfig{Port: 3000},
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: llmServer.URL,
				APIKey:   "super-secret-api-key-12345",
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

		// Verify API key is not in the response
		if strings.Contains(bodyStr, "super-secret-api-key-12345") {
			t.Error("health response should not contain API key")
		}
	})
}
