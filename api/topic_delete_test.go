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

// E2-S5: User deletes a topic

func TestDeleteTopic(t *testing.T) {
	t.Run("user can delete a topic and it is permanently removed", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		topicID := "topic-delete-001"
		store.Create(topicID, "Delete Me", "This topic will be deleted")

		req := httptest.NewRequest("DELETE", "/api/topics/"+topicID, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		// Verify topic is gone from the list
		listReq := httptest.NewRequest("GET", "/api/topics", nil)
		listResp, err := app.Test(listReq)
		if err != nil {
			t.Fatalf("failed to list topics: %v", err)
		}
		defer listResp.Body.Close()

		var topics []map[string]interface{}
		if err := json.NewDecoder(listResp.Body).Decode(&topics); err != nil {
			t.Fatalf("expected JSON array, got: %v", err)
		}

		if len(topics) != 0 {
			t.Errorf("expected 0 topics after deletion, got %d", len(topics))
		}
	})

	t.Run("deleting a topic removes its conversation history and requirement document", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		topicID := "topic-delete-002"
		topic := store.Create(topicID, "Full Delete", "Has messages and document")
		store.AddMessage(topicID, "user", "Important message")
		store.AddMessage(topicID, "assistant", "Important response")
		topic.Document = "# Requirements\n\n## Important Section\n\nThis is important content."

		req := httptest.NewRequest("DELETE", "/api/topics/"+topicID, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		resp.Body.Close()

		// Verify topic is completely gone
		detailReq := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp, err := app.Test(detailReq)
		if err != nil {
			t.Fatalf("failed to get topic detail: %v", err)
		}
		detailResp.Body.Close()

		if detailResp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404 after deletion, got %d", detailResp.StatusCode)
		}
	})

	t.Run("deleting one topic does not affect any other topic", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		// Create two topics
		topicA := "topic-delete-a"
		topicB := "topic-delete-b"
		store.Create(topicA, "Keep This", "Topic A should survive")
		store.Create(topicB, "Delete This", "Topic B will be deleted")
		store.AddMessage(topicA, "user", "Message in A")

		// Delete topic B
		req := httptest.NewRequest("DELETE", "/api/topics/"+topicB, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to delete topic B: %v", err)
		}
		resp.Body.Close()

		// Verify topic A is still intact
		detailReq := httptest.NewRequest("GET", "/api/topics/"+topicA, nil)
		detailResp, err := app.Test(detailReq)
		if err != nil {
			t.Fatalf("failed to get topic A: %v", err)
		}
		defer detailResp.Body.Close()

		if detailResp.StatusCode != http.StatusOK {
			t.Errorf("expected topic A to still exist, got status %d", detailResp.StatusCode)
		}

		var detail map[string]interface{}
		if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		if detail["name"] != "Keep This" {
			t.Errorf("expected topic A name 'Keep This', got %v", detail["name"])
		}

		// Verify topic A still has its message
		msgCount, ok := detail["message_count"].(float64)
		if !ok || msgCount != 1 {
			t.Errorf("expected topic A to have 1 message, got %v", detail["message_count"])
		}
	})

	t.Run("deleting a non-existent topic returns 404", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		req := httptest.NewRequest("DELETE", "/api/topics/non-existent", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})
}
