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

// E2-S7: User reopens a completed topic

func TestReopenTopic(t *testing.T) {
	t.Run("a completed topic can be reopened by the user", func(t *testing.T) {
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

		topicID := "topic-reopen-001"
		store.Create(topicID, "Reopen Me", "A topic to reopen")

		// Mark as completed
		completePayload := map[string]string{"status": "completed"}
		completeBody, _ := json.Marshal(completePayload)
		completeReq := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(completeBody))
		completeReq.Header.Set("Content-Type", "application/json")
		completeResp, _ := app.Test(completeReq)
		completeResp.Body.Close()

		if completeResp.StatusCode != http.StatusOK {
			t.Fatalf("failed to mark topic as completed: status %d", completeResp.StatusCode)
		}

		// Reopen the topic
		reopenPayload := map[string]string{"status": "active"}
		reopenBody, _ := json.Marshal(reopenPayload)
		reopenReq := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(reopenBody))
		reopenReq.Header.Set("Content-Type", "application/json")
		reopenResp, err := app.Test(reopenReq)
		if err != nil {
			t.Fatalf("failed to reopen topic: %v", err)
		}
		defer reopenResp.Body.Close()

		if reopenResp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 when reopening, got %d", reopenResp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(reopenResp.Body).Decode(&result); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		if result["status"] != "active" {
			t.Errorf("expected status 'active' after reopening, got %v", result["status"])
		}
	})

	t.Run("upon reopening the topic status returns to active", func(t *testing.T) {
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

		topicID := "topic-reopen-002"
		store.Create(topicID, "Status Check", "Testing status transition")

		// Complete
		completePayload := map[string]string{"status": "completed"}
		completeBody, _ := json.Marshal(completePayload)
		completeReq := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(completeBody))
		completeReq.Header.Set("Content-Type", "application/json")
		completeResp, _ := app.Test(completeReq)
		completeResp.Body.Close()

		// Reopen
		reopenPayload := map[string]string{"status": "active"}
		reopenBody, _ := json.Marshal(reopenPayload)
		reopenReq := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(reopenBody))
		reopenReq.Header.Set("Content-Type", "application/json")
		reopenResp, _ := app.Test(reopenReq)
		reopenResp.Body.Close()

		// Verify status via detail view
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

		if detail["status"] != "active" {
			t.Errorf("expected status 'active' after reopening, got %v", detail["status"])
		}
	})

	t.Run("the full conversation history and requirement document from before completion are preserved", func(t *testing.T) {
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

		topicID := "topic-reopen-003"
		topic := store.Create(topicID, "Preserve Data", "Testing data preservation")
		store.AddMessage(topicID, "user", "Original message 1")
		store.AddMessage(topicID, "assistant", "Original response 1")
		topic.Document = "# Original Document\n\n## Section 1\n\nContent here."

		// Mark as completed
		completePayload := map[string]string{"status": "completed"}
		completeBody, _ := json.Marshal(completePayload)
		completeReq := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(completeBody))
		completeReq.Header.Set("Content-Type", "application/json")
		completeResp, _ := app.Test(completeReq)
		completeResp.Body.Close()

		// Reopen
		reopenPayload := map[string]string{"status": "active"}
		reopenBody, _ := json.Marshal(reopenPayload)
		reopenReq := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(reopenBody))
		reopenReq.Header.Set("Content-Type", "application/json")
		reopenResp, _ := app.Test(reopenReq)
		reopenResp.Body.Close()

		// Verify conversation history is preserved
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

		// Check messages
		messages, ok := detail["messages"].([]interface{})
		if !ok || len(messages) != 2 {
			t.Errorf("expected 2 messages preserved, got %v", detail["messages"])
		}

		// Check document
		doc, ok := detail["document"].(string)
		if !ok || doc == "" {
			t.Error("expected document to be preserved after reopen")
		}
		if doc != "# Original Document\n\n## Section 1\n\nContent here." {
			t.Errorf("document content changed after reopen: %s", doc)
		}
	})

	t.Run("reopening does not create a new topic — it continues the existing one", func(t *testing.T) {
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

		topicID := "topic-reopen-004"
		store.Create(topicID, "Same Topic", "No new topic created")

		// Count topics before
		listReq1 := httptest.NewRequest("GET", "/api/topics", nil)
		listResp1, _ := app.Test(listReq1)
		var topics1 []map[string]interface{}
		json.NewDecoder(listResp1.Body).Decode(&topics1)
		listResp1.Body.Close()

		// Complete and reopen
		completePayload := map[string]string{"status": "completed"}
		completeBody, _ := json.Marshal(completePayload)
		completeReq := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(completeBody))
		completeReq.Header.Set("Content-Type", "application/json")
		completeResp, _ := app.Test(completeReq)
		completeResp.Body.Close()

		reopenPayload := map[string]string{"status": "active"}
		reopenBody, _ := json.Marshal(reopenPayload)
		reopenReq := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(reopenBody))
		reopenReq.Header.Set("Content-Type", "application/json")
		reopenResp, _ := app.Test(reopenReq)
		reopenResp.Body.Close()

		// Count topics after
		listReq2 := httptest.NewRequest("GET", "/api/topics", nil)
		listResp2, _ := app.Test(listReq2)
		var topics2 []map[string]interface{}
		json.NewDecoder(listResp2.Body).Decode(&topics2)
		listResp2.Body.Close()

		if len(topics1) != len(topics2) {
			t.Errorf("expected same number of topics after reopen (no new topic created), got %d before, %d after", len(topics1), len(topics2))
		}

		// Verify same ID
		if len(topics2) > 0 && topics2[0]["id"] != topicID {
			t.Errorf("expected same topic ID %s, got %v", topicID, topics2[0]["id"])
		}
	})

	t.Run("after reopening the user can add new messages to the topic", func(t *testing.T) {
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

		topicID := "topic-reopen-005"
		store.Create(topicID, "Message After Reopen", "Testing messages after reopen")

		// Complete
		completePayload := map[string]string{"status": "completed"}
		completeBody, _ := json.Marshal(completePayload)
		completeReq := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(completeBody))
		completeReq.Header.Set("Content-Type", "application/json")
		completeResp, _ := app.Test(completeReq)
		completeResp.Body.Close()

		// Reopen
		reopenPayload := map[string]string{"status": "active"}
		reopenBody, _ := json.Marshal(reopenPayload)
		reopenReq := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(reopenBody))
		reopenReq.Header.Set("Content-Type", "application/json")
		reopenResp, _ := app.Test(reopenReq)
		reopenResp.Body.Close()

		// Add a new message (should succeed now)
		msgPayload := map[string]string{"content": "New message after reopen"}
		msgBody, _ := json.Marshal(msgPayload)
		msgReq := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(msgBody))
		msgReq.Header.Set("Content-Type", "application/json")
		msgResp, err := app.Test(msgReq)
		if err != nil {
			t.Fatalf("failed to send message: %v", err)
		}
		defer msgResp.Body.Close()

		// Should succeed (200) — may or may not get LLM response, but shouldn't be 409
		if msgResp.StatusCode == http.StatusConflict {
			t.Error("expected message to be accepted after reopening, got 409 Conflict")
		}
	})
}
