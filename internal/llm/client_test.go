package llm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"wavelength/internal/config"
	"wavelength/internal/llm"
)

// E1-S5: System verifies LLM connectivity at startup

func TestLLMClientConnectivityCheck(t *testing.T) {
	t.Run("connectivity check succeeds when LLM endpoint is reachable", func(t *testing.T) {
		server := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
			checkRequestPath(t, r, "/chat/completions")

			resp := map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Hello!",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		})
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
				Endpoint: "http://localhost:59999",
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
		if err != nil && !strings.HasPrefix(err.Error(), "cannot connect to LLM service:") {
			t.Errorf("expected error message to start with 'cannot connect to LLM service:', got: %v", err)
		}
	})

	t.Run("connectivity check fails with clear error when credentials are invalid", func(t *testing.T) {
		server := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid api key"})
		})
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
		if err != nil && !strings.HasPrefix(err.Error(), "cannot connect to LLM service:") {
			t.Errorf("expected error message to start with 'cannot connect to LLM service:', got: %v", err)
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

		err := client.CheckConnectivity(ctx)
		if err == nil {
			t.Error("expected error for malformed endpoint, got nil")
		}
	})
}

func TestLLMClientCall(t *testing.T) {
	t.Run("non-streaming call returns assistant response", func(t *testing.T) {
		server := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
			checkRequestPath(t, r, "/chat/completions")
			w.Header().Set("Content-Type", "application/json")

			resp := map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "This is a test response.",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		})
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
		ctx := context.Background()

		response, err := client.Call(ctx, []llm.Message{
			{Role: "user", Content: "Hello"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if response != "This is a test response." {
			t.Errorf("expected 'This is a test response.', got %q", response)
		}
	})

	t.Run("non-streaming call returns error on server error", func(t *testing.T) {
		server := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "server error"})
		})
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
		ctx := context.Background()

		_, err := client.Call(ctx, []llm.Message{
			{Role: "user", Content: "Hello"},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "LLM service error") {
			t.Errorf("expected 'LLM service error' in error, got: %v", err)
		}
	})
}

func TestCheckWhisper(t *testing.T) {
	t.Run("whisper check succeeds when transcription endpoint is available", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/audio/transcriptions" {
				t.Errorf("expected path /v1/audio/transcriptions, got %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"text": "hello"})
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

		err := client.CheckWhisper(ctx)
		if err != nil {
			t.Errorf("expected no error for available endpoint, got: %v", err)
		}
	})

	t.Run("whisper check fails when transcription endpoint is unreachable", func(t *testing.T) {
		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: "http://localhost:59999",
				APIKey:   "test-key",
			},
		}

		client := llm.NewClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := client.CheckWhisper(ctx)
		if err == nil {
			t.Error("expected error for unreachable endpoint, got nil")
		}
	})

	t.Run("whisper check uses custom whisper_url when configured", func(t *testing.T) {
		var receivedPath string
		whisperServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"text": "transcribed"})
		}))
		defer whisperServer.Close()

		// LLM server should not be contacted for transcription
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("LLM server should not be called when whisper_url is set")
		}))
		defer llmServer.Close()

		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: llmServer.URL,
				APIKey:   "test-key",
			},
			Voice: config.VoiceConfig{
				WhisperURL: whisperServer.URL,
			},
		}

		client := llm.NewClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.CheckWhisper(ctx)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if receivedPath != "/v1/audio/transcriptions" {
			t.Errorf("expected path /v1/audio/transcriptions on whisper server, got %s", receivedPath)
		}
	})

	t.Run("whisper check succeeds with whispercpp server type", func(t *testing.T) {
		var receivedPath string
		var hasAuth bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			hasAuth = r.Header.Get("Authorization") != ""
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"text": []string{"hello world"}})
		}))
		defer server.Close()

		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: "http://localhost:59999",
				APIKey:   "test-key",
			},
			Voice: config.VoiceConfig{
				WhisperURL:  server.URL,
				WhisperType: "whispercpp",
			},
		}

		client := llm.NewClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.CheckWhisper(ctx)
		if err != nil {
			t.Errorf("expected no error for whispercpp endpoint, got: %v", err)
		}
		if receivedPath != "/inference" {
			t.Errorf("expected path /inference, got %s", receivedPath)
		}
		if hasAuth {
			t.Error("whispercpp should not send Authorization header")
		}
	})

	t.Run("whispercpp transcription returns text from array response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"text": []string{"Hello", " world"},
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: server.URL,
				APIKey:   "test-key",
			},
			Voice: config.VoiceConfig{
				WhisperURL:  server.URL,
				WhisperType: "whispercpp",
			},
		}

		client := llm.NewClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Send a silent WAV as audio data
		_, err := client.Transcribe(ctx, []byte{})
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("whispercpp fails when endpoint is unreachable", func(t *testing.T) {
		cfg := &config.Config{
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: "http://localhost:59999",
				APIKey:   "test-key",
			},
			Voice: config.VoiceConfig{
				WhisperURL:  "http://localhost:59998",
				WhisperType: "whispercpp",
			},
		}

		client := llm.NewClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := client.CheckWhisper(ctx)
		if err == nil {
			t.Error("expected error for unreachable whispercpp endpoint, got nil")
		}
	})
}

func TestLLMClientStreamResponse(t *testing.T) {
	t.Run("streaming returns token events then done", func(t *testing.T) {
		server := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
			checkRequestPath(t, r, "/chat/completions")
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")

			// Simulate SSE stream with two tokens
			fmt.Fprintf(w, "data: %s\n\n", toJSON(t, map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"delta": map[string]interface{}{
							"content": "Hello",
						},
					},
				},
			}))
			fmt.Fprintf(w, "data: %s\n\n", toJSON(t, map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"delta": map[string]interface{}{
							"content": " world!",
						},
					},
				},
			}))
			fmt.Fprintf(w, "data: [DONE]\n\n")
		})
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
		ctx := context.Background()

		var events []string
		pr, pw := io.Pipe()

		go func() {
			err := client.StreamResponse(ctx, pw, "You are a helpful assistant.", nil)
			pw.Close()
			if err != nil {
				t.Errorf("StreamResponse returned error: %v", err)
			}
		}()

		// Read all events from pipe
		decoder := json.NewDecoder(pr)
		for {
			var event map[string]interface{}
			if err := decoder.Decode(&event); err != nil {
				break
			}
			events = append(events, event["type"].(string))
		}

		if len(events) < 2 {
			t.Fatalf("expected at least 2 events (tokens + done), got %d: %v", len(events), events)
		}
		if events[len(events)-1] != "done" {
			t.Errorf("expected last event to be 'done', got %q", events[len(events)-1])
		}
	})
}

// --- Test helpers ---

// newOpenAITestServer creates a test server. The eino openai client appends
// /chat/completions to the base URL, so the handler should be set up to respond
// on any request path.
func newOpenAITestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
}

func checkRequestPath(t *testing.T, r *http.Request, expected string) {
	t.Helper()
	if r.URL.Path != expected {
		t.Errorf("expected request path %q, got %q", expected, r.URL.Path)
	}
}

func toJSON(t *testing.T, v interface{}) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return string(data)
}
