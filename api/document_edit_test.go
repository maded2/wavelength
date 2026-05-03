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

// E4-S6: User manually edits the requirement document

func TestEditDocument(t *testing.T) {
	t.Run("user can update the requirement document content", func(t *testing.T) {
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

		topicID := "topic-edit-001"
		store.Create(topicID, "Edit Doc", "Testing document editing")

		// Update the document
		payload := map[string]string{
			"document": "# Updated Requirements\n\n## New Section\n\nUser edited content.",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/api/topics/"+topicID+"/document", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		// Verify the document was updated
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

		if doc != "# Updated Requirements\n\n## New Section\n\nUser edited content." {
			t.Errorf("expected updated document, got: %s", doc)
		}
	})

	t.Run("user can modify any part of the document content", func(t *testing.T) {
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

		topicID := "topic-edit-002"
		topic := store.Create(topicID, "Modify Doc", "Testing modification")
		topic.Document = "# Original\n\n## Section 1\n\nOriginal content.\n\n## Section 2\n\nMore original."

		// Modify just one section
		payload := map[string]string{
			"document": "# Original\n\n## Section 1\n\nModified content.\n\n## Section 2\n\nMore original.",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/api/topics/"+topicID+"/document", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Verify only the intended section changed
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

		doc := detail["document"].(string)
		if doc != "# Original\n\n## Section 1\n\nModified content.\n\n## Section 2\n\nMore original." {
			t.Errorf("expected modified document, got: %s", doc)
		}
	})

	t.Run("user can add new content to the document", func(t *testing.T) {
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

		topicID := "topic-edit-003"
		topic := store.Create(topicID, "Add Content", "Testing content addition")
		topic.Document = "# Original\n\n## Section 1\n\nOriginal."

		// Add a new section
		payload := map[string]string{
			"document": "# Original\n\n## Section 1\n\nOriginal.\n\n## Section 2\n\nNew user-added section.",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/api/topics/"+topicID+"/document", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

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

		doc := detail["document"].(string)
		if doc != "# Original\n\n## Section 1\n\nOriginal.\n\n## Section 2\n\nNew user-added section." {
			t.Errorf("expected added content, got: %s", doc)
		}
	})

	t.Run("user can remove content from the document", func(t *testing.T) {
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

		topicID := "topic-edit-004"
		topic := store.Create(topicID, "Remove Content", "Testing content removal")
		topic.Document = "# Original\n\n## Section 1\n\nKeep this.\n\n## Section 2\n\nRemove this."

		// Remove a section
		payload := map[string]string{
			"document": "# Original\n\n## Section 1\n\nKeep this.",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/api/topics/"+topicID+"/document", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

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

		doc := detail["document"].(string)
		if doc != "# Original\n\n## Section 1\n\nKeep this." {
			t.Errorf("expected removed content, got: %s", doc)
		}
	})

	t.Run("editing one topics document does not affect another topics document", func(t *testing.T) {
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

		topicA := "topic-edit-a"
		topicB := "topic-edit-b"
		topicAObj := store.Create(topicA, "Topic A", "Requirement A")
		topicBObj := store.Create(topicB, "Topic B", "Requirement B")

		topicAObj.Document = "# Topic A\n\nOriginal A"
		topicBObj.Document = "# Topic B\n\nOriginal B"

		// Edit Topic A's document
		payload := map[string]string{
			"document": "# Topic A\n\nModified A",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/api/topics/"+topicA+"/document", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Verify Topic B's document is unchanged
		reqB := httptest.NewRequest("GET", "/api/topics/"+topicB, nil)
		respB, _ := app.Test(reqB)
		var detailB map[string]interface{}
		json.NewDecoder(respB.Body).Decode(&detailB)
		respB.Body.Close()

		docB := detailB["document"].(string)
		if docB != "# Topic B\n\nOriginal B" {
			t.Errorf("Topic B document was affected by Topic A edit, got: %s", docB)
		}
	})
}
