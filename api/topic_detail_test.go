package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/config"
	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// E2-S3: User views topic details and session information

func TestTopicDetail(t *testing.T) {
	t.Run("selecting a topic from the list displays its detail view", func(t *testing.T) {
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

		topicID := "topic-detail-001"
		store.Create(topicID, "Detail Topic", "A topic for testing details")

		req := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var detail map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		if detail["id"] != topicID {
			t.Errorf("expected topic ID %s, got %v", topicID, detail["id"])
		}
	})

	t.Run("the detail view shows topic name, description, creation date, last activity date, interview status, and message count", func(t *testing.T) {
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

		topicID := "topic-detail-002"
		topic := store.Create(topicID, "Full Detail Topic", "Testing all detail fields")
		topic.Status = "active"

		req := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var detail map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		// Check all required fields
		requiredFields := []string{"name", "description", "created_at", "updated_at", "status", "message_count"}
		for _, field := range requiredFields {
			if detail[field] == nil {
				t.Errorf("expected detail to contain field %q, got nil", field)
			}
		}

		if detail["name"] != "Full Detail Topic" {
			t.Errorf("expected name 'Full Detail Topic', got %v", detail["name"])
		}
		if detail["description"] != "Testing all detail fields" {
			t.Errorf("expected description to match, got %v", detail["description"])
		}
		if detail["status"] != "active" {
			t.Errorf("expected status 'active', got %v", detail["status"])
		}
	})

	t.Run("the detail view provides access points to continue interview, view document, and view history", func(t *testing.T) {
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

		topicID := "topic-detail-003"
		store.Create(topicID, "Access Points Topic", "Testing access points")

		req := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var detail map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		// The detail response should include the messages (conversation history)
		// and document fields, even if empty
		if detail["messages"] == nil {
			t.Error("expected detail to include messages field for conversation history")
		}
		if detail["document"] == nil {
			t.Error("expected detail to include document field")
		}
	})

	t.Run("if the topic has not started an interview the detail view indicates this", func(t *testing.T) {
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

		topicID := "topic-detail-004"
		store.Create(topicID, "Not Started Topic", "No interview yet")

		req := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var detail map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		if detail["status"] != "not_started" {
			t.Errorf("expected status 'not_started' for topic without interview, got %v", detail["status"])
		}

		// Message count should be 0
		msgCount, ok := detail["message_count"].(float64)
		if !ok || msgCount != 0 {
			t.Errorf("expected message_count 0, got %v", detail["message_count"])
		}
	})

	t.Run("non-existent topic returns 404", func(t *testing.T) {
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

		req := httptest.NewRequest("GET", "/api/topics/non-existent-topic", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})
}
