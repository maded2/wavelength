package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/config"
	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// E3-S9: Interview conversations are fully isolated between topics

func TestTopicIsolation(t *testing.T) {
	t.Run("the AI agent in Topic A has no access to Topic Bs conversation history", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		// Capture what the LLM receives for each topic
		var topicBPrompt string
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := new(bytes.Buffer)
			io.Copy(buf, r.Body)
			body := buf.String()
			// Determine which topic based on the prompt content
			if !strings.Contains(body, "Topic A") {
				topicBPrompt = body
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Response."}}]}`))
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

		// Create Topic A with specific conversation
		topicAID := "topic-isolation-a"
		store.Create(topicAID, "Topic A", "Building a banking application")
		store.AddMessage(topicAID, "user", "We need two-factor authentication")

		// Create Topic B with different conversation
		topicBID := "topic-isolation-b"
		store.Create(topicBID, "Topic B", "Building a gaming platform")

		// Send message to Topic B
		payload := map[string]string{"content": "What about in-game purchases?"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicBID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Verify Topic B's LLM prompt does NOT contain Topic A's conversation
		if strings.Contains(topicBPrompt, "two-factor authentication") {
			t.Error("Topic B's LLM prompt should not contain Topic A's conversation")
		}
		if strings.Contains(topicBPrompt, "banking") {
			t.Error("Topic B's LLM prompt should not contain Topic A's requirement")
		}
	})

	t.Run("starting an interview in Topic A does not surface information from Topic B", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		var receivedPrompt string
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := new(bytes.Buffer)
			io.Copy(buf, r.Body)
			receivedPrompt = buf.String()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Response."}}]}`))
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

		// Create Topic B with sensitive data
		topicBID := "topic-isolation-b2"
		store.Create(topicBID, "Topic B", "Internal HR system with salary data")
		store.AddMessage(topicBID, "user", "We need to store employee salaries")

		// Create Topic A (fresh)
		topicAID := "topic-isolation-a2"
		store.Create(topicAID, "Topic A", "Public website for a restaurant")

		// Start interview in Topic A
		payload := map[string]string{"content": "Let's start the interview"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicAID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Verify Topic A's LLM prompt does NOT contain Topic B's data
		if strings.Contains(receivedPrompt, "salary") {
			t.Error("Topic A's LLM prompt should not contain Topic B's salary data")
		}
		if strings.Contains(receivedPrompt, "Internal HR system") || strings.Contains(receivedPrompt, "employee salaries") {
			t.Error("Topic A's LLM prompt should not contain Topic B's HR context")
		}
	})

	t.Run("the agents questions in each topic are based solely on that topics requirement", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		var receivedPrompt string
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := new(bytes.Buffer)
			io.Copy(buf, r.Body)
			receivedPrompt = buf.String()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Response."}}]}`))
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

		topicID := "topic-isolation-single"
		store.Create(topicID, "Single Topic", "A medical records system")

		payload := map[string]string{"content": "Start interview"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Verify the prompt contains the topic's own requirement
		if !strings.Contains(receivedPrompt, "medical records") {
			t.Error("LLM prompt should contain the topic's own requirement")
		}
		if !strings.Contains(receivedPrompt, "Single Topic") {
			t.Error("LLM prompt should contain the topic's name")
		}
	})

	t.Run("switching from Topic A to Topic B does not affect Topic Bs interview", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Response."}}]}`))
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

		// Create Topic A and B
		topicAID := "topic-switch-a"
		topicBID := "topic-switch-b"
		store.Create(topicAID, "Topic A", "Requirement A")
		store.Create(topicBID, "Topic B", "Requirement B")

		// Add messages to Topic A
		store.AddMessage(topicAID, "user", "Message in A")

		// "Switch" to Topic B and add message
		payload := map[string]string{"content": "Message in B"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicBID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Verify Topic A is unchanged
		topicA := store.Get(topicAID)
		if topicA == nil {
			t.Fatal("Topic A not found")
		}
		if len(topicA.Messages) != 1 {
			t.Errorf("expected Topic A to have 1 message, got %d", len(topicA.Messages))
		}
		if topicA.Messages[0].Content != "Message in A" {
			t.Error("Topic A message was modified by switching to Topic B")
		}

		// Verify Topic B has its own messages only
		topicB := store.Get(topicBID)
		if topicB == nil {
			t.Fatal("Topic B not found")
		}
		if len(topicB.Messages) < 1 {
			t.Error("Topic B should have at least 1 message")
		}
		if topicB.Messages[0].Content != "Message in B" {
			t.Error("Topic B first message should be its own")
		}
	})

	t.Run("isolation is maintained when both topics are used in close succession", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Response."}}]}`))
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

		topicAID := "topic-rapid-a"
		topicBID := "topic-rapid-b"
		store.Create(topicAID, "Rapid A", "Fast Topic A")
		store.Create(topicBID, "Rapid B", "Fast Topic B")

		// Rapid succession: A, B, A, B
		exchanges := []struct {
			topicID string
			content string
		}{
			{topicAID, "A message 1"},
			{topicBID, "B message 1"},
			{topicAID, "A message 2"},
			{topicBID, "B message 2"},
		}

		for _, ex := range exchanges {
			payload := map[string]string{"content": ex.content}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/topics/"+ex.topicID+"/messages", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := app.Test(req)
			resp.Body.Close()
		}

		// Verify Topic A has only A messages
		topicA := store.Get(topicAID)
		if topicA == nil {
			t.Fatal("Topic A not found")
		}
		for _, msg := range topicA.Messages {
			if msg.Role == "user" && strings.Contains(msg.Content, "B message") {
				t.Error("Topic A contains a message from Topic B")
			}
		}

		// Verify Topic B has only B messages
		topicB := store.Get(topicBID)
		if topicB == nil {
			t.Fatal("Topic B not found")
		}
		for _, msg := range topicB.Messages {
			if msg.Role == "user" && strings.Contains(msg.Content, "A message") {
				t.Error("Topic B contains a message from Topic A")
			}
		}
	})
}
