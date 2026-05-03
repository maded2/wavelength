package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/config"
	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// E3-S6: User views the full conversation history for a topic

func TestConversationHistory(t *testing.T) {
	t.Run("the conversation history displays all messages in chronological order", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Agent response."}}]}`))
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
		SetupRoutes(app, store, client)

		topicID := "topic-history-001"
		store.Create(topicID, "History Test", "Testing conversation history")

		// Create multiple exchanges
		for i := 0; i < 3; i++ {
			payload := map[string]string{"content": "User message " + string(rune('0'+i))}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := app.Test(req)
			resp.Body.Close()
		}

		// Get topic detail with messages
		detailReq := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp, err := app.Test(detailReq)
		if err != nil {
			t.Fatalf("failed to get detail: %v", err)
		}
		defer detailResp.Body.Close()

		var detail map[string]interface{}
		if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		messages, ok := detail["messages"].([]interface{})
		if !ok {
			t.Fatal("expected messages to be an array")
		}

		// Should have 6 messages (3 user + 3 assistant)
		if len(messages) != 6 {
			t.Errorf("expected 6 messages, got %d", len(messages))
		}

		// Verify chronological order
		for i := 0; i < len(messages); i++ {
			msg := messages[i].(map[string]interface{})
			expectedRole := "user"
			if i%2 == 1 {
				expectedRole = "assistant"
			}
			if msg["role"] != expectedRole {
				t.Errorf("message %d: expected role %q, got %v", i, expectedRole, msg["role"])
			}
		}
	})

	t.Run("messages are clearly labeled with role and timestamp", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Agent response."}}]}`))
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
		SetupRoutes(app, store, client)

		topicID := "topic-history-002"
		store.Create(topicID, "Labels Test", "Testing message labels")

		payload := map[string]string{"content": "Test message"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Get topic detail
		detailReq := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp, err := app.Test(detailReq)
		if err != nil {
			t.Fatalf("failed to get detail: %v", err)
		}
		defer detailResp.Body.Close()

		var detail map[string]interface{}
		if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		messages, ok := detail["messages"].([]interface{})
		if !ok {
			t.Fatal("expected messages to be an array")
		}

		for i, msgInterface := range messages {
			msg := msgInterface.(map[string]interface{})

			// Check role
			if msg["role"] == nil || msg["role"] == "" {
				t.Errorf("message %d: expected role to be set", i)
			}

			// Check timestamp
			if msg["timestamp"] == nil || msg["timestamp"] == "" {
				t.Errorf("message %d: expected timestamp to be set", i)
			}

			// Check content
			if msg["content"] == nil || msg["content"] == "" {
				t.Errorf("message %d: expected content to be set", i)
			}
		}
	})

	t.Run("the history is available for long conversations with many exchanges", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Agent response."}}]}`))
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
		SetupRoutes(app, store, client)

		topicID := "topic-history-003"
		store.Create(topicID, "Long History", "Testing long conversation")

		// Create 20 exchanges
		for i := 0; i < 20; i++ {
			payload := map[string]string{"content": "User message " + string(rune('0'+i%10))}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := app.Test(req)
			resp.Body.Close()
			time.Sleep(1 * time.Millisecond) // Ensure distinct timestamps
		}

		// Get topic detail
		detailReq := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp, err := app.Test(detailReq)
		if err != nil {
			t.Fatalf("failed to get detail: %v", err)
		}
		defer detailResp.Body.Close()

		var detail map[string]interface{}
		if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		messages, ok := detail["messages"].([]interface{})
		if !ok {
			t.Fatal("expected messages to be an array")
		}

		// Should have 40 messages (20 user + 20 assistant)
		if len(messages) != 40 {
			t.Errorf("expected 40 messages, got %d", len(messages))
		}

		// Verify message count field
		msgCount, ok := detail["message_count"].(float64)
		if !ok || int(msgCount) != 40 {
			t.Errorf("expected message_count 40, got %v", detail["message_count"])
		}
	})

	t.Run("the history view is read-only", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		cfg := &config.Config{
			Server: config.ServerConfig{Port: 3000},
			LLM: config.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				Endpoint: "http://localhost:11434",
				APIKey:   "test-key",
			},
			DataDir: t.TempDir(),
		}
		client := llm.NewClient(cfg)
		SetupRoutes(app, store, client)

		topicID := "topic-history-004"
		store.Create(topicID, "Read Only Test", "Testing read-only history")
		store.AddMessage(topicID, "user", "Original message")

		// Try to modify the topic via PUT (should not be supported)
		modPayload := map[string]interface{}{
			"name": "Modified Name",
		}
		modBody, _ := json.Marshal(modPayload)
		modReq := httptest.NewRequest("PUT", "/api/topics/"+topicID, bytes.NewReader(modBody))
		modReq.Header.Set("Content-Type", "application/json")
		modResp, err := app.Test(modReq)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		modResp.Body.Close()

		// PUT should not be allowed (404 or 405)
		if modResp.StatusCode == http.StatusOK || modResp.StatusCode == http.StatusCreated {
			t.Errorf("expected PUT to not be supported, got status %d", modResp.StatusCode)
		}

		// Verify the original message is unchanged
		detailReq := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp, err := app.Test(detailReq)
		if err != nil {
			t.Fatalf("failed to get detail: %v", err)
		}
		defer detailResp.Body.Close()

		var detail map[string]interface{}
		if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		messages, ok := detail["messages"].([]interface{})
		if !ok {
			t.Fatal("expected messages to be an array")
		}

		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}

		msg := messages[0].(map[string]interface{})
		if msg["content"] != "Original message" {
			t.Errorf("expected original message to be unchanged, got %v", msg["content"])
		}
	})
}
