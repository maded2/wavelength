package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/config"
	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// E3-S8: System manages long conversations approaching LLM context limits

func TestContextManagement(t *testing.T) {
	t.Run("long conversations do not produce technical context limit errors", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusRequestEntityTooLarge)
			w.Write([]byte(`{"error":"context length exceeded"}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-context-001"
		store.Create(topicID, "Long Conv", "Testing context management")

		payload := map[string]string{"content": "This is a message in a long conversation"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Should still return 200 (graceful handling)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 for graceful handling, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		// Response should NOT contain raw technical errors
		respBody, _ := json.Marshal(result)
		respStr := string(respBody)
		if strings.Contains(strings.ToLower(respStr), "context length") ||
			strings.Contains(strings.ToLower(respStr), "token limit") ||
			strings.Contains(strings.ToLower(respStr), "413") {
			t.Errorf("expected no raw technical context errors in response, got: %s", respStr)
		}
	})

	t.Run("the user sees a user-friendly message when context management occurs", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusRequestEntityTooLarge)
			w.Write([]byte(`{"error":"context length exceeded"}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-context-002"
		store.Create(topicID, "Friendly Error", "Testing friendly error messages")

		payload := map[string]string{"content": "User message in long conversation"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		// Should indicate the agent is unavailable (graceful degradation)
		if result["success"] != false {
			t.Errorf("expected success=false when LLM fails, got: %v", result["success"])
		}

		errMsg, ok := result["error"].(string)
		if !ok || errMsg == "" {
			t.Error("expected user-friendly error message in response")
		}

		lowerErr := strings.ToLower(errMsg)
		// Should be user-friendly, not technical
		if strings.Contains(lowerErr, "token") || strings.Contains(lowerErr, "context length") {
			t.Errorf("expected user-friendly error, got technical message: %s", errMsg)
		}
	})

	t.Run("the users message is preserved even when context limits are reached", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusRequestEntityTooLarge)
			w.Write([]byte(`{"error":"context length exceeded"}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-context-003"
		store.Create(topicID, "Preserve Msg", "Testing message preservation")

		userMsg := "Important decision: we need SSO support"
		payload := map[string]string{"content": userMsg}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Verify user message was saved despite LLM failure
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

		if len(messages) == 0 {
			t.Fatal("expected user message to be preserved, got 0 messages")
		}

		firstMsg := messages[0].(map[string]interface{})
		if firstMsg["content"] != userMsg {
			t.Errorf("expected user message to be preserved, got %v", firstMsg["content"])
		}
	})

	t.Run("the conversation state remains intact for resumption after context issues", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"choices":[{"message":{"content":"Got it."}}]}`))
			} else {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				w.Write([]byte(`{"error":"context length exceeded"}`))
			}
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-context-004"
		store.Create(topicID, "Resume After", "Testing resume after context issue")

		// First message works
		payload1 := map[string]string{"content": "First message"}
		body1, _ := json.Marshal(payload1)
		req1 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body1))
		req1.Header.Set("Content-Type", "application/json")
		resp1, _ := app.Test(req1)
		resp1.Body.Close()

		// Second message hits context limit
		payload2 := map[string]string{"content": "Second message - context limit"}
		body2, _ := json.Marshal(payload2)
		req2 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, _ := app.Test(req2)
		resp2.Body.Close()

		// Verify all messages are preserved for resumption
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

		// Should have 3 messages: user1, assistant1, user2
		if len(messages) < 3 {
			t.Errorf("expected at least 3 messages preserved, got %d", len(messages))
		}

		// Verify both user messages are intact
		userMsgs := 0
		for _, msg := range messages {
			if msg.(map[string]interface{})["role"] == "user" {
				userMsgs++
			}
		}
		if userMsgs != 2 {
			t.Errorf("expected 2 user messages preserved, got %d", userMsgs)
		}
	})
}
