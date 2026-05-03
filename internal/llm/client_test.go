package llm_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"wavelength/internal/config"
	"wavelength/internal/llm"
)

// E1-S5: System verifies LLM connectivity at startup

func TestLLMClientConnectivityCheck(t *testing.T) {
	t.Run("connectivity check succeeds when LLM endpoint is reachable", func(t *testing.T) {
		// Create a test server that responds to connectivity checks
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: server.URL,
				APIKey:   "test-key",
			},
		}

		client := llm.NewClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.CheckConnectivity(ctx)
		if err != nil {
			t.Errorf("expected no error for reachable endpoint, got: %v", err)
		}
	})

	t.Run("connectivity check fails with clear error when endpoint is unreachable", func(t *testing.T) {
		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: "http://localhost:59999", // non-existent server
				APIKey:   "test-key",
			},
		}

		client := llm.NewClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := client.CheckConnectivity(ctx)
		if err == nil {
			t.Error("expected error for unreachable endpoint, got nil")
		}
		errMsg := err.Error()
		if len(errMsg) == 0 {
			t.Error("expected descriptive error message for connectivity failure")
		}
	})

	t.Run("connectivity check fails with clear error when credentials are invalid", func(t *testing.T) {
		// Create a test server that returns 401 for invalid credentials
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid api key"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: server.URL,
				APIKey:   "invalid-key",
			},
		}

		client := llm.NewClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.CheckConnectivity(ctx)
		if err == nil {
			t.Error("expected error for invalid credentials, got nil")
		}
	})

	t.Run("connectivity check does not panic or hang on malformed endpoint", func(t *testing.T) {
		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: "not-a-valid-url",
				APIKey:   "test-key",
			},
		}

		client := llm.NewClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Should return an error, not panic
		err := client.CheckConnectivity(ctx)
		if err == nil {
			t.Error("expected error for malformed endpoint, got nil")
		}
	})
}
