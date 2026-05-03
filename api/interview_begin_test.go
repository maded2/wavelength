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

// E3-S1: User begins an interview for a topic

func TestBeginInterview(t *testing.T) {
	t.Run("user can initiate the interview process from a topic", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		// Mock LLM server that returns a business analyst introduction
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"choices": [{
					"message": {
						"content": "Hello! I'm your business analyst. I understand you want to build an online store for handmade crafts. Let me start by asking: who are the primary users of this platform?"
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

		topicID := "topic-interview-001"
		store.Create(topicID, "Craft Store", "An online store for selling handmade crafts")

		// Start interview by sending first message
		payload := map[string]string{
			"content": "/start",
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

		// Should have assistant response
		if result["success"] != true {
			t.Errorf("expected successful interview start, got: %v", result)
		}
	})

	t.Run("the AI agent introduces itself as a business analyst", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"choices": [{
					"message": {
						"content": "Hello! I'm your business analyst working for the IT department. I'd like to help you gather requirements for your project."
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

		topicID := "topic-interview-002"
		store.Create(topicID, "Test App", "A test application")

		payload := map[string]string{"content": "/start"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)

		// Response should contain business analyst introduction
		lowerBody := strings.ToLower(bodyStr)
		if !strings.Contains(lowerBody, "business analyst") && !strings.Contains(lowerBody, "analyst") {
			t.Errorf("expected agent to introduce itself as a business analyst, got: %s", bodyStr)
		}
	})

	t.Run("the agents first message references the high-level requirement", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		// Verify the LLM receives the topic description in the prompt
		receivedPrompt := ""
		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read the request body to check it contains the requirement
			buf := new(strings.Builder)
			io.Copy(buf, r.Body)
			receivedPrompt = buf.String()

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"choices": [{
					"message": {
						"content": "I see you want to build an inventory management system. Let me ask about the key features."
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

		topicID := "topic-interview-003"
		store.Create(topicID, "Inventory System", "A system to manage warehouse inventory with barcode scanning")

		payload := map[string]string{"content": "/start"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		resp.Body.Close()

		// Verify the LLM received the high-level requirement in its prompt
		if !strings.Contains(receivedPrompt, "inventory") {
			t.Errorf("expected LLM prompt to reference the requirement 'inventory', got: %s", receivedPrompt)
		}
	})

	t.Run("if no high-level requirement is provided the agent asks the user to describe their idea", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"choices": [{
					"message": {
						"content": "I don't have a description of what you want to build yet. Could you please describe your idea?"
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

		topicID := "topic-interview-004"
		// Create topic with empty description (edge case)
		store.Create(topicID, "Untitled Idea", "")

		payload := map[string]string{"content": "/start"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)

		// Response should ask user to describe their idea
		lowerBody := strings.ToLower(bodyStr)
		if !strings.Contains(lowerBody, "describe") && !strings.Contains(lowerBody, "idea") && !strings.Contains(lowerBody, "what") {
			t.Errorf("expected agent to ask user to describe their idea, got: %s", bodyStr)
		}
	})

	t.Run("the conversation is displayed with clear message ownership", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"choices": [{
					"message": {
						"content": "Let me ask about your requirements."
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

		topicID := "topic-interview-005"
		store.Create(topicID, "Chat Test", "Testing chat format")

		payload := map[string]string{"content": "Hello, I want to start"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		resp.Body.Close()

		// Check the topic detail for message roles
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

		// Should have at least user and assistant messages with roles
		if len(messages) < 2 {
			t.Fatalf("expected at least 2 messages (user + assistant), got %d", len(messages))
		}

		// First message should be from user
		firstMsg := messages[0].(map[string]interface{})
		if firstMsg["role"] != "user" {
			t.Errorf("expected first message role 'user', got %v", firstMsg["role"])
		}

		// Second message should be from assistant
		secondMsg := messages[1].(map[string]interface{})
		if secondMsg["role"] != "assistant" {
			t.Errorf("expected second message role 'assistant', got %v", secondMsg["role"])
		}
	})
}
