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
		app, llmServer := newHealthTestApp(t, true)
		defer llmServer.Close()

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
		app := newHealthTestAppUnreachable(t)

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

		// Reason should be in plain language, not raw technical errors
		lowerReason := strings.ToLower(reason)
		hasPlainLanguage := strings.Contains(lowerReason, "cannot connect") ||
			strings.Contains(lowerReason, "unavailable") ||
			strings.Contains(lowerReason, "error")
		if !hasPlainLanguage {
			t.Errorf("expected plain language reason, got: %s", reason)
		}
	})

	t.Run("health endpoint does not expose sensitive data like API keys", func(t *testing.T) {
		app, llmServer := newHealthTestApp(t, true)
		defer llmServer.Close()

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

	t.Run("health status reflects real-time LLM availability", func(t *testing.T) {
		// Server that starts unavailable, then becomes available
		available := false
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if available {
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
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
		app := fiber.New()
		app.Get("/health", HealthHandler(client, false))

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

// newHealthTestApp creates a Fiber app with a health endpoint wired to a mock LLM server.
// When available is true, the mock responds 200; when false, it responds 503.
func newHealthTestApp(t *testing.T, available bool) (*fiber.App, *httptest.Server) {
	t.Helper()
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if available {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))

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
	app := fiber.New()
	app.Get("/health", HealthHandler(client, false))
	return app, llmServer
}

// newHealthTestAppUnreachable creates a Fiber app with a health endpoint pointing to a non-existent LLM.
func newHealthTestAppUnreachable(t *testing.T) *fiber.App {
	t.Helper()
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
	app := fiber.New()
	app.Get("/health", HealthHandler(client, false))
	return app
}
