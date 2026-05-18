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
	"github.com/google/uuid"
	"wavelength/internal/config"
	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// E1-S6: System handles LLM unavailability gracefully during an interview

func TestLLMFailureDuringInterview(t *testing.T) {
	t.Run("user sees a clear non-technical message when the AI agent cannot respond", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"service unavailable"}`))
		})
		app := suite.App
		store := suite.Store

		// Create a topic first
		topicID := uuid.New().String()
		store.Create(topicID, "Test Topic", "A test requirement")

		// Submit a user message
		payload := map[string]string{
			"topic_id": topicID,
			"content":  "I want a login page with email and password",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Should return 200 (not an error) even though LLM failed
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		respBody, _ := io.ReadAll(resp.Body)
		respStr := string(respBody)

		// Response should indicate the agent is unavailable in user-friendly terms
		if !strings.Contains(strings.ToLower(respStr), "unavailable") &&
			!strings.Contains(strings.ToLower(respStr), "temporarily") &&
			!strings.Contains(strings.ToLower(respStr), "error") {
			t.Errorf("expected user-friendly error message, got: %s", respStr)
		}
	})

	t.Run("the users message is preserved even when the LLM fails", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusServiceUnavailable)
		})
		app := suite.App
		store := suite.Store

		topicID := uuid.New().String()
		store.Create(topicID, "Test Topic", "A test requirement")

		userMessage := "I need a dashboard with charts"
		payload := map[string]string{
			"topic_id": topicID,
			"content":  userMessage,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		resp.Body.Close()

		// Check that the user's message was saved
		topic := store.Get(topicID)
		if topic == nil {
			t.Fatal("topic not found after message submission")
		}

		found := false
		for _, msg := range topic.Messages {
			if msg.Role == "user" && msg.Content == userMessage {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected user message to be preserved in conversation history despite LLM failure")
		}
	})

	t.Run("the entire conversation history up to that point remains intact after LLM failure", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
callCount++
			if callCount <= 2 {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"choices":[{"message":{"content":"Got it, let me ask about that."}}]}`))
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		})
		app := suite.App
		store := suite.Store

		topicID := uuid.New().String()
		store.Create(topicID, "Test Topic", "A test requirement")

		// First message (LLM works)
		payload1 := map[string]string{
			"topic_id": topicID,
			"content":  "First message",
		}
		body1, _ := json.Marshal(payload1)
		req1 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body1))
		req1.Header.Set("Content-Type", "application/json")
		resp1, _ := app.Test(req1)
		resp1.Body.Close()

		// Second message (LLM fails)
		payload2 := map[string]string{
			"topic_id": topicID,
			"content":  "Second message",
		}
		body2, _ := json.Marshal(payload2)
		req2 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, _ := app.Test(req2)
		resp2.Body.Close()

		// Check that both user messages are preserved
		topic := store.Get(topicID)
		if topic == nil {
			t.Fatal("topic not found")
		}

		userMessages := 0
		for _, msg := range topic.Messages {
			if msg.Role == "user" {
				userMessages++
			}
		}
		if userMessages < 2 {
			t.Errorf("expected at least 2 user messages preserved, got %d", userMessages)
		}
	})

	t.Run("an LLM failure in one topic does not affect the state of other topics", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusServiceUnavailable)
		})
		app := suite.App
		store := suite.Store

		// Create two topics
		topicA := uuid.New().String()
		topicB := uuid.New().String()
		store.Create(topicA, "Topic A", "Requirement A")
		store.Create(topicB, "Topic B", "Requirement B")

		// Pre-populate Topic B with messages
		store.AddMessage(topicB, "user", "Hello from Topic B")
		store.AddMessage(topicB, "assistant", "Response from Topic B")

		// Submit a message to Topic A (LLM fails)
		payload := map[string]string{
			"topic_id": topicA,
			"content":  "Message for Topic A",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicA+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Verify Topic B is unaffected
		topicBData := store.Get(topicB)
		if topicBData == nil {
			t.Fatal("Topic B not found")
		}

		if len(topicBData.Messages) != 2 {
			t.Errorf("expected Topic B to have 2 messages, got %d", len(topicBData.Messages))
		}

		// Verify Topic B messages are intact
		if topicBData.Messages[0].Content != "Hello from Topic B" {
			t.Error("Topic B first message was modified")
		}
	})
}
