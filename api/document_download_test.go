package api

import (
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

func TestDownloadDocument(t *testing.T) {
	t.Run("download markdown format returns correct content type and data", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()
		cfg := &config.Config{
			Server:  config.ServerConfig{Port: 3000},
			LLM:     config.LLMConfig{Provider: "openai", Model: "gpt-4", Endpoint: "http://localhost:11434", APIKey: "test-key"},
			DataDir: t.TempDir(),
		}
		client := llm.NewClient(cfg)
		SetupRoutes(app, store, client)

		topicID := "topic-dl-001"
		tp := store.Create(topicID, "My Topic", "Test topic")
		tp.Document = "# Test\n\nContent here."

		req := httptest.NewRequest("GET", "/api/topics/"+topicID+"/document/download?format=markdown", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType != "text/markdown" {
			t.Errorf("expected content type text/markdown, got %s", contentType)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "# Test\n\nContent here." {
			t.Errorf("expected markdown content, got: %s", string(body))
		}
	})

	t.Run("download pdf format returns valid PDF header", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()
		cfg := &config.Config{
			Server:  config.ServerConfig{Port: 3000},
			LLM:     config.LLMConfig{Provider: "openai", Model: "gpt-4", Endpoint: "http://localhost:11434", APIKey: "test-key"},
			DataDir: t.TempDir(),
		}
		client := llm.NewClient(cfg)
		SetupRoutes(app, store, client)

		topicID := "topic-dl-002"
		tp := store.Create(topicID, "PDF Topic", "Test PDF")
		tp.Document = "# PDF Test\n\nSome content."

		req := httptest.NewRequest("GET", "/api/topics/"+topicID+"/document/download?format=pdf", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType != "application/pdf" {
			t.Errorf("expected content type application/pdf, got %s", contentType)
		}

		body, _ := io.ReadAll(resp.Body)
		if len(body) < 5 || string(body[:5]) != "%PDF-" {
			t.Errorf("expected valid PDF header, got: %s", string(body[:min(len(body), 20)]))
		}
	})

	t.Run("download word format returns valid DOCX header", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()
		cfg := &config.Config{
			Server:  config.ServerConfig{Port: 3000},
			LLM:     config.LLMConfig{Provider: "openai", Model: "gpt-4", Endpoint: "http://localhost:11434", APIKey: "test-key"},
			DataDir: t.TempDir(),
		}
		client := llm.NewClient(cfg)
		SetupRoutes(app, store, client)

		topicID := "topic-dl-003"
		tp := store.Create(topicID, "Word Topic", "Test Word")
		tp.Document = "# Word Test\n\nContent."

		req := httptest.NewRequest("GET", "/api/topics/"+topicID+"/document/download?format=word", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
			t.Errorf("expected word mime type, got %s", contentType)
		}

		body, _ := io.ReadAll(resp.Body)
		if len(body) < 2 || string(body[:2]) != "PK" {
			t.Errorf("expected valid DOCX (ZIP) header, got: %s", string(body[:min(len(body), 20)]))
		}
	})

	t.Run("download defaults to markdown when no format specified", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()
		cfg := &config.Config{
			Server:  config.ServerConfig{Port: 3000},
			LLM:     config.LLMConfig{Provider: "openai", Model: "gpt-4", Endpoint: "http://localhost:11434", APIKey: "test-key"},
			DataDir: t.TempDir(),
		}
		client := llm.NewClient(cfg)
		SetupRoutes(app, store, client)

		topicID := "topic-dl-004"
		tp := store.Create(topicID, "Default Format", "Test default")
		tp.Document = "# Default\n\nMarkdown."

		req := httptest.NewRequest("GET", "/api/topics/"+topicID+"/document/download", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType != "text/markdown" {
			t.Errorf("expected default content type text/markdown, got %s", contentType)
		}
	})

	t.Run("download returns 404 for non-existent topic", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()
		cfg := &config.Config{
			Server:  config.ServerConfig{Port: 3000},
			LLM:     config.LLMConfig{Provider: "openai", Model: "gpt-4", Endpoint: "http://localhost:11434", APIKey: "test-key"},
			DataDir: t.TempDir(),
		}
		client := llm.NewClient(cfg)
		SetupRoutes(app, store, client)

		req := httptest.NewRequest("GET", "/api/topics/nonexistent/document/download", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		if result["message"] != "topic not found" {
			t.Errorf("expected 'topic not found' message, got: %v", result["message"])
		}
	})

	t.Run("download returns 400 for unsupported format", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()
		cfg := &config.Config{
			Server:  config.ServerConfig{Port: 3000},
			LLM:     config.LLMConfig{Provider: "openai", Model: "gpt-4", Endpoint: "http://localhost:11434", APIKey: "test-key"},
			DataDir: t.TempDir(),
		}
		client := llm.NewClient(cfg)
		SetupRoutes(app, store, client)

		topicID := "topic-dl-005"
		store.Create(topicID, "Bad Format", "Test bad format")

		req := httptest.NewRequest("GET", "/api/topics/"+topicID+"/document/download?format=rtf", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		if result["message"] == nil {
			t.Error("expected error message for unsupported format")
		}
	})

	t.Run("download sets Content-Disposition header for file download", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()
		cfg := &config.Config{
			Server:  config.ServerConfig{Port: 3000},
			LLM:     config.LLMConfig{Provider: "openai", Model: "gpt-4", Endpoint: "http://localhost:11434", APIKey: "test-key"},
			DataDir: t.TempDir(),
		}
		client := llm.NewClient(cfg)
		SetupRoutes(app, store, client)

		topicID := "topic-dl-006"
		store.Create(topicID, "Download Header Test", "Test headers")

		req := httptest.NewRequest("GET", "/api/topics/"+topicID+"/document/download", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		disposition := resp.Header.Get("Content-Disposition")
		if disposition == "" {
			t.Error("expected Content-Disposition header to be set")
		}
		if !strings.HasPrefix(disposition, "attachment") {
			t.Errorf("expected attachment disposition, got: %s", disposition)
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
