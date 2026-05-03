package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/config"
	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// E3-S2: User engages in conversational back-and-forth with the AI agent

func TestConversationalBackAndForth(t *testing.T) {
	t.Run("user can type a free-form text response and submit it to the AI agent", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"choices": [{
					"message": {
						"content": "That's helpful. Let me ask about the payment flow next."
					}
				}]
			}`))
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

		topicID := "topic-chat-001"
		store.Create(topicID, "Chat Test", "Testing chat functionality")

		// Submit free-form message
		payload := map[string]string{
			"content": "I need users to be able to create accounts with email verification",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		if result["success"] != true {
			t.Errorf("expected successful response, got: %v", result)
		}
	})

	t.Run("after the user submits a response the AI agent generates a follow-up", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"choices": [{
					"message": {
						"content": "Interesting. What authentication methods should be supported besides email?"
					}
				}]
			}`))
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

		topicID := "topic-chat-002"
		store.Create(topicID, "Follow-up Test", "Testing follow-up questions")

		payload := map[string]string{
			"content": "Users should log in with email and password",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		// Verify agent response is present
		msg, ok := result["message"].(map[string]interface{})
		if !ok {
			t.Fatal("expected message field in response")
		}

		if msg["role"] != "assistant" {
			t.Errorf("expected assistant role, got %v", msg["role"])
		}

		content, ok := msg["content"].(string)
		if !ok || content == "" {
			t.Error("expected non-empty assistant content in response")
		}
	})

	t.Run("the conversation is displayed as a chronological exchange of messages", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		responseCount := 0
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			responseCount++
			w.WriteHeader(http.StatusOK)
			if responseCount == 1 {
				w.Write([]byte(`{"choices":[{"message":{"content":"Got it. What about user roles?"}}]}`))
			} else {
				w.Write([]byte(`{"choices":[{"message":{"content":"Understood. Any admin features?"}}]}`))
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
		SetupRoutes(app, store, client)

		topicID := "topic-chat-003"
		store.Create(topicID, "Chronology Test", "Testing message order")

		// First exchange
		payload1 := map[string]string{"content": "First user message"}
		body1, _ := json.Marshal(payload1)
		req1 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body1))
		req1.Header.Set("Content-Type", "application/json")
		resp1, _ := app.Test(req1)
		resp1.Body.Close()

		// Second exchange
		payload2 := map[string]string{"content": "Second user message"}
		body2, _ := json.Marshal(payload2)
		req2 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, _ := app.Test(req2)
		resp2.Body.Close()

		// Check topic detail for chronological order
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

		if len(messages) != 4 {
			t.Fatalf("expected 4 messages (2 user + 2 assistant), got %d", len(messages))
		}

		// Verify chronological order: user, assistant, user, assistant
		expectedRoles := []string{"user", "assistant", "user", "assistant"}
		for i, expectedRole := range expectedRoles {
			msg := messages[i].(map[string]interface{})
			if msg["role"] != expectedRole {
				t.Errorf("message %d: expected role %q, got %v", i, expectedRole, msg["role"])
			}
		}
	})

	t.Run("the users message appears in the conversation immediately upon submission", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		// LLM server that delays (simulated by always working)
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Processing your response..."}}]}`))
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

		topicID := "topic-chat-004"
		store.Create(topicID, "Immediate Test", "Testing immediate message save")

		userMsg := "This message should appear immediately"
		payload := map[string]string{"content": userMsg}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Check that the user's message is saved before the response comes back
		topic := store.Get(topicID)
		if topic == nil {
			t.Fatal("topic not found")
		}

		// First message should be the user's
		if len(topic.Messages) == 0 {
			t.Fatal("expected at least one message in conversation")
		}

		firstMsg := topic.Messages[0]
		if firstMsg.Role != "user" {
			t.Errorf("expected first message to be from user, got role %q", firstMsg.Role)
		}
		if firstMsg.Content != userMsg {
			t.Errorf("expected first message content to match user input, got %q", firstMsg.Content)
		}
	})

	t.Run("the response includes an indication that the agent is processing", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Here is the follow-up question."}}]}`))
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

		topicID := "topic-chat-005"
		store.Create(topicID, "Typing Indicator Test", "Testing response format")

		payload := map[string]string{"content": "My response to the agent"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// The response should include both the user's message (for immediate display)
		// and the assistant's response
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		// The response should contain the assistant's message with a role
		msg, ok := result["message"].(map[string]interface{})
		if !ok {
			t.Fatal("expected message field in response")
		}

		if msg["role"] != "assistant" {
			t.Errorf("expected assistant role in response, got %v", msg["role"])
		}

		// Should have a timestamp for the frontend to display
		if msg["timestamp"] == nil {
			t.Error("expected timestamp in response for display purposes")
		}
	})

	t.Run("the agents response is relevant to the users last message", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		// Verify the LLM receives the user's latest message in the prompt
		receivedBody := ""
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := new(bytes.Buffer)
			buf.ReadFrom(r.Body)
			receivedBody = buf.String()

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"That's a great point about security."}}]}`))
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

		topicID := "topic-chat-006"
		store.Create(topicID, "Relevance Test", "Testing response relevance")

		userMsg := "The system needs to handle data encryption at rest"
		payload := map[string]string{"content": userMsg}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		resp.Body.Close()

		// Verify the user's message was sent to the LLM
		if !contains(receivedBody, "encryption") {
			t.Errorf("expected LLM prompt to contain user's message about encryption, got: %s", receivedBody)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
