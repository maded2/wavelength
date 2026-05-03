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

// E4-S1: User views the current requirement document for a topic

func TestViewRequirementDocument(t *testing.T) {
	t.Run("the requirement document is accessible from the topic detail view", func(t *testing.T) {
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

		topicID := "topic-doc-001"
		store.Create(topicID, "Doc View Test", "Testing document view")

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

		// Document field should be present
		if detail["document"] == nil {
			t.Error("expected document field in topic detail")
		}
	})

	t.Run("the document is displayed in readable markdown format", func(t *testing.T) {
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

		topicID := "topic-doc-002"
		topic := store.Create(topicID, "Markdown Test", "Testing markdown format")
		topic.Document = "# Requirements\n\n## Overview\n\nThis is a test document.\n\n## Users\n\n- Admin\n- Customer"

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

		doc, ok := detail["document"].(string)
		if !ok {
			t.Fatal("expected document to be a string")
		}

		// Verify markdown structure is preserved
		if doc != "# Requirements\n\n## Overview\n\nThis is a test document.\n\n## Users\n\n- Admin\n- Customer" {
			t.Errorf("expected markdown document to be preserved, got: %s", doc)
		}
	})

	t.Run("the document reflects all requirement information elicited up to that point", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Noted."}}]}`))
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

		topicID := "topic-doc-003"
		topic := store.Create(topicID, "Elicited Doc", "Testing elicited content")
		// Simulate document being updated during interview
		topic.Document = "# Requirements\n\n## Functional\n\nUser can log in."

		// Add a message
		payload := map[string]string{"content": "Also need password reset"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Check document is accessible
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

		doc, ok := detail["document"].(string)
		if !ok {
			t.Fatal("expected document to be a string")
		}

		if doc == "" {
			t.Error("expected document to contain elicited requirements")
		}
	})

	t.Run("the document is clearly associated with its topic", func(t *testing.T) {
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

		topicID := "topic-doc-004"
		topic := store.Create(topicID, "Association Test", "Testing topic association")
		topic.Document = "# Requirements for Association Test"

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

		// Topic name should be present alongside document
		if detail["name"] != "Association Test" {
			t.Errorf("expected topic name in response, got %v", detail["name"])
		}
		if detail["document"] == "" {
			t.Error("expected document to be present in topic detail")
		}
	})

	t.Run("if no requirement information has been documented yet the view indicates this", func(t *testing.T) {
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

		topicID := "topic-doc-005"
		store.Create(topicID, "Empty Doc", "Testing empty document state")

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

		// Document field should be present but empty
		doc, ok := detail["document"].(string)
		if !ok {
			t.Fatal("expected document field to be a string")
		}

		if doc != "" {
			t.Errorf("expected empty document for new topic, got: %q", doc)
		}
	})

	t.Run("the document view updates when the user navigates to it", func(t *testing.T) {
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

		topicID := "topic-doc-006"
		topic := store.Create(topicID, "Update Test", "Testing document updates")

		// First fetch — empty document
		req1 := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		resp1, _ := app.Test(req1)
		var detail1 map[string]interface{}
		json.NewDecoder(resp1.Body).Decode(&detail1)
		resp1.Body.Close()

		if detail1["document"] != "" {
			t.Error("expected empty document initially")
		}

		// Update document directly
		topic.Document = "# Updated Requirements\n\n## New Section\n\nUpdated content."

		// Second fetch — should see updated document
		req2 := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		resp2, _ := app.Test(req2)
		var detail2 map[string]interface{}
		json.NewDecoder(resp2.Body).Decode(&detail2)
		resp2.Body.Close()

		doc, ok := detail2["document"].(string)
		if !ok {
			t.Fatal("expected document to be a string")
		}
		if doc != "# Updated Requirements\n\n## New Section\n\nUpdated content." {
			t.Errorf("expected updated document, got: %s", doc)
		}
	})
}
