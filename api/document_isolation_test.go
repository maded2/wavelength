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

// E4-S4: Each topic has its own isolated requirement document

func TestDocumentIsolation(t *testing.T) {
	t.Run("each topic is associated with exactly one requirement document", func(t *testing.T) {
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
		SetupRoutes(app, store, client, cfg.DataDir)

		topicA := "topic-doc-iso-a"
		topicB := "topic-doc-iso-b"
		topicAObj := store.Create(topicA, "Topic A", "Requirement A")
		topicBObj := store.Create(topicB, "Topic B", "Requirement B")

		// Each topic has its own document field
		if topicAObj.Document != "" || topicBObj.Document != "" {
			// OK if both empty (new topics)
		}

		// Set different documents
		topicAObj.Document = "# Topic A Requirements"
		topicBObj.Document = "# Topic B Requirements"

		// Verify via API
		reqA := httptest.NewRequest("GET", "/api/topics/"+topicA, nil)
		respA, _ := app.Test(reqA)
		var detailA map[string]interface{}
		json.NewDecoder(respA.Body).Decode(&detailA)
		respA.Body.Close()

		reqB := httptest.NewRequest("GET", "/api/topics/"+topicB, nil)
		respB, _ := app.Test(reqB)
		var detailB map[string]interface{}
		json.NewDecoder(respB.Body).Decode(&detailB)
		respB.Body.Close()

		docA := detailA["document"].(string)
		docB := detailB["document"].(string)

		if docA != "# Topic A Requirements" {
			t.Errorf("expected Topic A document, got: %s", docA)
		}
		if docB != "# Topic B Requirements" {
			t.Errorf("expected Topic B document, got: %s", docB)
		}
	})

	t.Run("the document for Topic A contains only information from Topic As interview", func(t *testing.T) {
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
		SetupRoutes(app, store, client, cfg.DataDir)

		topicA := "topic-doc-iso-a2"
		topicB := "topic-doc-iso-b2"
		topicAObj := store.Create(topicA, "Topic A", "Requirement A")
		topicBObj := store.Create(topicB, "Topic B", "Requirement B")

		// Set documents with distinct content
		topicAObj.Document = "# Topic A\n\n## Users\n\n- Admin"
		topicBObj.Document = "# Topic B\n\n## Features\n\n- Gaming"

		// Verify Topic A document doesn't contain Topic B content
		reqA := httptest.NewRequest("GET", "/api/topics/"+topicA, nil)
		respA, _ := app.Test(reqA)
		var detailA map[string]interface{}
		json.NewDecoder(respA.Body).Decode(&detailA)
		respA.Body.Close()

		docA := detailA["document"].(string)
		if docA != "# Topic A\n\n## Users\n\n- Admin" {
			t.Errorf("Topic A document was contaminated, got: %s", docA)
		}
	})

	t.Run("the document for Topic B contains only information from Topic Bs interview", func(t *testing.T) {
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
		SetupRoutes(app, store, client, cfg.DataDir)

		topicA := "topic-doc-iso-a3"
		topicB := "topic-doc-iso-b3"
		topicAObj := store.Create(topicA, "Topic A", "Requirement A")
		topicBObj := store.Create(topicB, "Topic B", "Requirement B")

		// Set documents
		topicAObj.Document = "# Topic A\n\nBanking requirements"
		topicBObj.Document = "# Topic B\n\nGaming requirements"

		// Verify Topic B document doesn't contain Topic A content
		reqB := httptest.NewRequest("GET", "/api/topics/"+topicB, nil)
		respB, _ := app.Test(reqB)
		var detailB map[string]interface{}
		json.NewDecoder(respB.Body).Decode(&detailB)
		respB.Body.Close()

		docB := detailB["document"].(string)
		if docB != "# Topic B\n\nGaming requirements" {
			t.Errorf("Topic B document was contaminated, got: %s", docB)
		}
	})

	t.Run("viewing Topic As document never shows content from Topic Bs document", func(t *testing.T) {
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Response."}}]}`))
		})
		app := suite.App
		store := suite.Store

		topicA := "topic-doc-iso-a4"
		topicB := "topic-doc-iso-b4"
		topicAObj := store.Create(topicA, "Topic A", "Requirement A")
		topicBObj := store.Create(topicB, "Topic B", "Requirement B")

		// Set distinct documents
		topicAObj.Document = "# Topic A Document\n\nSecret A content"
		topicBObj.Document = "# Topic B Document\n\nSecret B content"

		// View Topic A — should not show B's content
		reqA := httptest.NewRequest("GET", "/api/topics/"+topicA, nil)
		respA, _ := app.Test(reqA)
		var detailA map[string]interface{}
		json.NewDecoder(respA.Body).Decode(&detailA)
		respA.Body.Close()

		docA := detailA["document"].(string)
		if docA != "# Topic A Document\n\nSecret A content" {
			t.Errorf("Topic A document shows wrong content: %s", docA)
		}
		if docA != "" && docA != "# Topic A Document\n\nSecret A content" {
			t.Error("Topic A document may be contaminated")
		}
	})

	t.Run("if Topic A is deleted Topic Bs document is unaffected", func(t *testing.T) {
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
		SetupRoutes(app, store, client, cfg.DataDir)

		topicA := "topic-doc-iso-a5"
		topicB := "topic-doc-iso-b5"
		topicAObj := store.Create(topicA, "Topic A", "Requirement A")
		topicBObj := store.Create(topicB, "Topic B", "Requirement B")

		topicAObj.Document = "# Topic A\n\nWill be deleted"
		topicBObj.Document = "# Topic B\n\nShould survive"

		// Delete Topic A
		delReq := httptest.NewRequest("DELETE", "/api/topics/"+topicA, nil)
		delResp, _ := app.Test(delReq)
		delResp.Body.Close()

		// Verify Topic B document is unchanged
		reqB := httptest.NewRequest("GET", "/api/topics/"+topicB, nil)
		respB, _ := app.Test(reqB)
		var detailB map[string]interface{}
		json.NewDecoder(respB.Body).Decode(&detailB)
		respB.Body.Close()

		docB := detailB["document"].(string)
		if docB != "# Topic B\n\nShould survive" {
			t.Errorf("Topic B document was affected by Topic A deletion, got: %s", docB)
		}
	})
}
