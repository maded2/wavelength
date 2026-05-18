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

// E3-S7: User pauses and resumes an interview session

func TestPauseAndResumeInterview(t *testing.T) {
	t.Run("the user can navigate away from an interview and the conversation state is preserved", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Agent response."}}]}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-pause-001"
		store.Create(topicID, "Pause Test", "Testing pause and resume")

		// Have a few exchanges
		for i := 0; i < 3; i++ {
			payload := map[string]string{"content": "User message " + string(rune('0'+i))}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := app.Test(req)
			resp.Body.Close()
		}

		// "Navigate away" — just fetch topic detail to simulate returning
		detailReq := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp, err := app.Test(detailReq)
		if err != nil {
			t.Fatalf("failed to get detail: %v", err)
		}
		defer detailResp.Body.Close()

		if detailResp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", detailResp.StatusCode)
		}

		var detail map[string]interface{}
		if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		// Verify all messages are still there
		messages, ok := detail["messages"].([]interface{})
		if !ok {
			t.Fatal("expected messages to be an array")
		}

		if len(messages) != 6 {
			t.Errorf("expected 6 messages preserved after pause, got %d", len(messages))
		}
	})

	t.Run("when the user returns the full conversation history is displayed", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Agent response."}}]}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-pause-002"
		store.Create(topicID, "Resume Test", "Testing resume with history")

		// Create conversation
		payload1 := map[string]string{"content": "What kind of authentication do you need?"}
		body1, _ := json.Marshal(payload1)
		req1 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body1))
		req1.Header.Set("Content-Type", "application/json")
		resp1, _ := app.Test(req1)
		resp1.Body.Close()

		payload2 := map[string]string{"content": "Email and password with OAuth"}
		body2, _ := json.Marshal(payload2)
		req2 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, _ := app.Test(req2)
		resp2.Body.Close()

		// "Return" to the topic
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

		// All 4 messages should be present
		if len(messages) != 4 {
			t.Errorf("expected 4 messages on resume, got %d", len(messages))
		}

		// Verify the conversation content is intact
		if messages[0].(map[string]interface{})["content"] != "What kind of authentication do you need?" {
			t.Error("first message content not preserved")
		}
		if messages[2].(map[string]interface{})["content"] != "Email and password with OAuth" {
			t.Error("third message content not preserved")
		}
	})

	t.Run("the AI agent is aware of the full prior conversation history when resuming", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
buf := new(bytes.Buffer)
			buf.ReadFrom(r.Body)
			receivedPrompt = buf.String()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"I see, let me continue."}}]}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-pause-003"
		store.Create(topicID, "Context Test", "Testing context on resume")

		// First exchange
		payload1 := map[string]string{"content": "I need a login system"}
		body1, _ := json.Marshal(payload1)
		req1 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body1))
		req1.Header.Set("Content-Type", "application/json")
		resp1, _ := app.Test(req1)
		resp1.Body.Close()

		// "Pause" — do nothing

		// Resume with new message
		payload2 := map[string]string{"content": "Also need password reset"}
		body2, _ := json.Marshal(payload2)
		req2 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, _ := app.Test(req2)
		resp2.Body.Close()

		// Verify the LLM received the prior conversation context
		if !contains(receivedPrompt, "login system") {
			t.Errorf("expected LLM prompt to include prior conversation about login system, got: %s", receivedPrompt)
		}
	})

	t.Run("the user can see the last messages to reorient themselves", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Agent response."}}]}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-pause-004"
		store.Create(topicID, "Reorient Test", "Testing reorientation")

		// Create conversation
		for i := 0; i < 3; i++ {
			payload := map[string]string{"content": "Message " + string(rune('0'+i))}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := app.Test(req)
			resp.Body.Close()
		}

		// "Return" and check detail
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

		// Should have all messages including the last ones
		if len(messages) < 4 {
			t.Errorf("expected at least 4 messages for reorientation, got %d", len(messages))
		}

		// The last message should be from the assistant (most recent exchange)
		lastMsg := messages[len(messages)-1].(map[string]interface{})
		if lastMsg["role"] != "assistant" {
			t.Errorf("expected last message to be from assistant, got role %v", lastMsg["role"])
		}
	})

	t.Run("resuming does not require any special save action", func(t *testing.T) {
		app := fiber.New()
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Agent response."}}]}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-pause-005"
		store.Create(topicID, "Auto Save Test", "Testing auto-save")

		// Send a message
		payload := map[string]string{"content": "Auto-saved message"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Immediately fetch the topic — no explicit save needed
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

		// Messages should be there without any explicit save
		if len(messages) == 0 {
			t.Error("expected messages to be automatically saved without explicit save action")
		}

		// First message should be the user's
		firstMsg := messages[0].(map[string]interface{})
		if firstMsg["content"] != "Auto-saved message" {
			t.Errorf("expected auto-saved message, got %v", firstMsg["content"])
		}
	})
}
