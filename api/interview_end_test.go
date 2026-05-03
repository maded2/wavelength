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

// E3-S11: User manually ends the interview

func TestEndInterview(t *testing.T) {
	t.Run("user can signal the agent that they are done with the interview", func(t *testing.T) {
		app := fiber.New()
		store := topic.NewStore()

		llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Great, I'll wrap up."}}]}`))
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

		topicID := "topic-end-001"
		store.Create(topicID, "End Interview", "Testing interview end")

		// Signal end of interview
		payload := map[string]string{"status": "completed"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(body))
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

		if result["status"] != "completed" {
			t.Errorf("expected status 'completed', got %v", result["status"])
		}
	})

	t.Run("after concluding the topic transitions to completed state", func(t *testing.T) {
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

		topicID := "topic-end-002"
		store.Create(topicID, "Transition Test", "Testing state transition")

		// Mark as completed
		payload := map[string]string{"status": "completed"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Verify via detail view
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

		if detail["status"] != "completed" {
			t.Errorf("expected topic status 'completed', got %v", detail["status"])
		}
	})

	t.Run("the completed topic can be reopened later", func(t *testing.T) {
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

		topicID := "topic-end-003"
		store.Create(topicID, "Reopen Test", "Testing reopen after complete")

		// Complete
		payload1 := map[string]string{"status": "completed"}
		body1, _ := json.Marshal(payload1)
		req1 := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(body1))
		req1.Header.Set("Content-Type", "application/json")
		resp1, _ := app.Test(req1)
		resp1.Body.Close()

		// Reopen
		payload2 := map[string]string{"status": "active"}
		body2, _ := json.Marshal(payload2)
		req2 := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, _ := app.Test(req2)
		resp2.Body.Close()

		// Verify reopened
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
			t.Errorf("expected topic status 'active' after reopen, got %v", detail["status"])
		}
	})

	t.Run("the conclusion is a deliberate user-driven action", func(t *testing.T) {
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

		topicID := "topic-end-004"
		store.Create(topicID, "Deliberate End", "Testing deliberate conclusion")

		// Topic should start as not_started
		detailReq := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp, _ := app.Test(detailReq)
		var detail map[string]interface{}
		json.NewDecoder(detailResp.Body).Decode(&detail)
		detailResp.Body.Close()

		if detail["status"] != "not_started" {
			t.Errorf("expected initial status 'not_started', got %v", detail["status"])
		}

		// Send a message (topic becomes active)
		payload := map[string]string{"content": "Let's start"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Topic should now be active, not completed
		detailReq2 := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp2, _ := app.Test(detailReq2)
		var detail2 map[string]interface{}
		json.NewDecoder(detailResp2.Body).Decode(&detail2)
		detailResp2.Body.Close()

		if detail2["status"] != "active" {
			t.Errorf("expected status 'active' after message, got %v", detail2["status"])
		}

		// Only explicit PATCH to completed should change it
		payload2 := map[string]string{"status": "completed"}
		body2, _ := json.Marshal(payload2)
		req2 := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, _ := app.Test(req2)
		resp2.Body.Close()

		detailReq3 := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp3, _ := app.Test(detailReq3)
		var detail3 map[string]interface{}
		json.NewDecoder(detailResp3.Body).Decode(&detail3)
		detailResp3.Body.Close()

		if detail3["status"] != "completed" {
			t.Errorf("expected status 'completed' after explicit PATCH, got %v", detail3["status"])
		}
	})
}
