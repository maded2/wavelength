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

// E2-S1: User creates a new requirement-gathering topic

func TestCreateTopic(t *testing.T) {
	t.Run("user can create a topic by providing a name and high-level description", func(t *testing.T) {
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

		payload := map[string]string{
			"name":        "E-Commerce Platform",
			"description": "An online store for selling handmade crafts",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/topics", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("expected status 201 Created, got %d", resp.StatusCode)
		}

		var created map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		if created["name"] != "E-Commerce Platform" {
			t.Errorf("expected topic name 'E-Commerce Platform', got %v", created["name"])
		}
		if created["description"] != "An online store for selling handmade crafts" {
			t.Errorf("expected description to match, got %v", created["description"])
		}
		if created["id"] == "" || created["id"] == nil {
			t.Error("expected topic to have an ID")
		}
	})

	t.Run("topic name is required with a clear error message", func(t *testing.T) {
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

		payload := map[string]string{
			"name":        "",
			"description": "Some description",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/topics", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}

		respBody, _ := io.ReadAll(resp.Body)
		respStr := string(respBody)

		// Error message should mention that name is required
		lowerResp := strings.ToLower(respStr)
		if !strings.Contains(lowerResp, "name") || !strings.Contains(lowerResp, "required") {
			t.Errorf("expected error message about required name, got: %s", respStr)
		}
	})

	t.Run("high-level description is required with explanation of its purpose", func(t *testing.T) {
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

		payload := map[string]string{
			"name":        "My Topic",
			"description": "",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/topics", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}

		respBody, _ := io.ReadAll(resp.Body)
		respStr := string(respBody)

		// Error message should explain that description is what the AI agent uses to begin the interview
		lowerResp := strings.ToLower(respStr)
		if !strings.Contains(lowerResp, "description") || !strings.Contains(lowerResp, "required") {
			t.Errorf("expected error message about required description, got: %s", respStr)
		}
		if !strings.Contains(lowerResp, "agent") && !strings.Contains(lowerResp, "interview") && !strings.Contains(lowerResp, "ai") {
			t.Errorf("expected error message to explain the purpose of the description, got: %s", respStr)
		}
	})

	t.Run("upon creation the topic appears in the user's topic list", func(t *testing.T) {
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

		// Create a topic
		payload := map[string]string{
			"name":        "New Feature",
			"description": "A new feature for the app",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// List topics
		req2 := httptest.NewRequest("GET", "/api/topics", nil)
		resp2, err := app.Test(req2)
		if err != nil {
			t.Fatalf("failed to list topics: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp2.StatusCode)
		}

		var topics []map[string]interface{}
		if err := json.NewDecoder(resp2.Body).Decode(&topics); err != nil {
			t.Fatalf("expected JSON array, got: %v", err)
		}

		if len(topics) != 1 {
			t.Fatalf("expected 1 topic in list, got %d", len(topics))
		}

		if topics[0]["name"] != "New Feature" {
			t.Errorf("expected topic name 'New Feature' in list, got %v", topics[0]["name"])
		}
	})

	t.Run("each newly created topic is independent and isolated from all other topics", func(t *testing.T) {
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

		// Create two topics
		topicA := map[string]string{
			"name":        "Topic A",
			"description": "Requirements for Topic A",
		}
		topicB := map[string]string{
			"name":        "Topic B",
			"description": "Requirements for Topic B",
		}

		bodyA, _ := json.Marshal(topicA)
		reqA := httptest.NewRequest("POST", "/api/topics", bytes.NewReader(bodyA))
		reqA.Header.Set("Content-Type", "application/json")
		respA, _ := app.Test(reqA)
		respA.Body.Close()

		bodyB, _ := json.Marshal(topicB)
		reqB := httptest.NewRequest("POST", "/api/topics", bytes.NewReader(bodyB))
		reqB.Header.Set("Content-Type", "application/json")
		respB, _ := app.Test(reqB)
		respB.Body.Close()

		// List topics and verify they are separate
		reqList := httptest.NewRequest("GET", "/api/topics", nil)
		respList, err := app.Test(reqList)
		if err != nil {
			t.Fatalf("failed to list topics: %v", err)
		}
		defer respList.Body.Close()

		var topics []map[string]interface{}
		if err := json.NewDecoder(respList.Body).Decode(&topics); err != nil {
			t.Fatalf("expected JSON array, got: %v", err)
		}

		if len(topics) != 2 {
			t.Fatalf("expected 2 topics, got %d", len(topics))
		}

		// Verify each topic has its own unique ID
		ids := make(map[string]bool)
		for _, tp := range topics {
			id := tp["id"].(string)
			if ids[id] {
				t.Error("expected each topic to have a unique ID")
			}
			ids[id] = true
		}

		// Verify descriptions are not mixed
		for _, tp := range topics {
			name := tp["name"].(string)
			desc := tp["description"].(string)
			if name == "Topic A" && !strings.Contains(desc, "Topic A") {
				t.Error("Topic A has wrong description")
			}
			if name == "Topic B" && !strings.Contains(desc, "Topic B") {
				t.Error("Topic B has wrong description")
			}
		}
	})

	t.Run("duplicate topic name is rejected with guidance to choose a different name", func(t *testing.T) {
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

		// Create first topic
		payload1 := map[string]string{
			"name":        "My Project",
			"description": "First description",
		}
		body1, _ := json.Marshal(payload1)
		req1 := httptest.NewRequest("POST", "/api/topics", bytes.NewReader(body1))
		req1.Header.Set("Content-Type", "application/json")
		resp1, _ := app.Test(req1)
		resp1.Body.Close()

		// Try to create topic with same name
		payload2 := map[string]string{
			"name":        "My Project",
			"description": "Second description",
		}
		body2, _ := json.Marshal(payload2)
		req2 := httptest.NewRequest("POST", "/api/topics", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := app.Test(req2)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusConflict {
			t.Errorf("expected status 409 Conflict, got %d", resp2.StatusCode)
		}

		respBody, _ := io.ReadAll(resp2.Body)
		respStr := string(respBody)

		// Error message should inform about duplicate name
		lowerResp := strings.ToLower(respStr)
		if !strings.Contains(lowerResp, "already exists") && !strings.Contains(lowerResp, "duplicate") {
			t.Errorf("expected error about existing topic, got: %s", respStr)
		}
		if !strings.Contains(lowerResp, "different") || !strings.Contains(lowerResp, "name") {
			t.Errorf("expected guidance to choose a different name, got: %s", respStr)
		}
	})
}
