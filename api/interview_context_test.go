package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// E3-S10: User provides additional context beyond the initial high-level requirement

func TestAdditionalContext(t *testing.T) {
	t.Run("user can provide additional context at any point during the interview", func(t *testing.T) {
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Understood, GDPR compliance noted."}}]}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-context-001"
		store.Create(topicID, "GDPR App", "A customer management system")

		// First exchange
		payload1 := map[string]string{"content": "We need to track customer interactions"}
		body1, _ := json.Marshal(payload1)
		req1 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body1))
		req1.Header.Set("Content-Type", "application/json")
		resp1, _ := app.Test(req1)
		resp1.Body.Close()

		// Provide additional context mid-interview
		payload2 := map[string]string{"content": "By the way, this needs to comply with GDPR"}
		body2, _ := json.Marshal(payload2)
		req2 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := app.Test(req2)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp2.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		if result["success"] != true {
			t.Errorf("expected successful response, got: %v", result)
		}
	})

	t.Run("the additional context is reflected in the conversation history", func(t *testing.T) {
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Noted."}}]}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-context-002"
		store.Create(topicID, "Context History", "Testing context in history")

		// Provide context as a regular message
		payload := map[string]string{"content": "This is for a healthcare setting"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Check topic detail
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

		// Find the context message in history
		found := false
		for _, msg := range messages {
			m := msg.(map[string]interface{})
			if m["role"] == "user" && m["content"] == "This is for a healthcare setting" {
				found = true
				break
			}
		}

		if !found {
			t.Error("expected additional context message to be in conversation history")
		}
	})

	t.Run("the AI agent receives the additional context in the conversation prompt", func(t *testing.T) {
		var receivedPrompt string
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
			buf := new(bytes.Buffer)
			io.Copy(buf, r.Body)
			receivedPrompt = buf.String()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Understood."}}]}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-context-003"
		store.Create(topicID, "Prompt Context", "Testing prompt includes context")

		// First message
		payload1 := map[string]string{"content": "We need a login system"}
		body1, _ := json.Marshal(payload1)
		req1 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body1))
		req1.Header.Set("Content-Type", "application/json")
		resp1, _ := app.Test(req1)
		resp1.Body.Close()

		// Additional context message
		payload2 := map[string]string{"content": "Also, this must integrate with our existing LDAP"}
		body2, _ := json.Marshal(payload2)
		req2 := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, _ := app.Test(req2)
		resp2.Body.Close()

		// Verify the LLM received both messages in the conversation context
		if !strings.Contains(receivedPrompt, "login system") {
			t.Error("expected LLM prompt to include first message")
		}
		if !strings.Contains(receivedPrompt, "LDAP") {
			t.Error("expected LLM prompt to include additional context about LDAP")
		}
	})

	t.Run("no special command or format is required to provide context", func(t *testing.T) {
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Understood."}}]}`))
		})
		app := suite.App
		store := suite.Store

		topicID := "topic-context-004"
		store.Create(topicID, "No Special Format", "Testing plain text context")

		// Just type context as a normal message — no special syntax
		payload := map[string]string{"content": "Just typing this as a normal message about our domain"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 for plain text context, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		if result["success"] != true {
			t.Errorf("expected successful response for plain text context, got: %v", result)
		}
	})
}
