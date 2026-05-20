package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// E2-S6: User marks a topic as complete

func TestMarkTopicComplete(t *testing.T) {
	t.Run("user can mark a topic as complete", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		topicID := "topic-complete-001"
		store.Create(topicID, "Complete Me", "A topic to complete")

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

	t.Run("a completed topic is visually distinguished from active topics in the topic list", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		// Create two topics
		activeID := "topic-active-001"
		completedID := "topic-completed-001"
		store.Create(activeID, "Active Topic", "Still working on this")
		store.Create(completedID, "Completed Topic", "Done with this")

		// Mark one as completed
		payload := map[string]string{"status": "completed"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/api/topics/"+completedID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// List topics and verify statuses differ
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

		if len(topics) != 2 {
			t.Fatalf("expected 2 topics, got %d", len(topics))
		}

		// Find the completed topic
		foundCompleted := false
		foundActive := false
		for _, tp := range topics {
			if tp["status"] == "completed" {
				foundCompleted = true
			}
			if tp["status"] == "active" || tp["status"] == "not_started" {
				foundActive = true
			}
		}

		if !foundCompleted {
			t.Error("expected a topic with status 'completed' in the list")
		}
		if !foundActive {
			t.Error("expected a non-completed topic in the list")
		}
	})

	t.Run("the requirement document for a completed topic remains viewable", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		topicID := "topic-complete-002"
		topic := store.Create(topicID, "Doc Topic", "Has a document")
		topic.Document = "# Requirements\n\n## Overview\n\nFinal requirements document."

		// Mark as completed
		payload := map[string]string{"status": "completed"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Verify document is still accessible via detail view
		detailReq := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp, err := app.Test(detailReq)
		if err != nil {
			t.Fatalf("failed to get topic detail: %v", err)
		}
		defer detailResp.Body.Close()

		if detailResp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 for completed topic detail, got %d", detailResp.StatusCode)
		}

		var detail map[string]interface{}
		if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		if detail["document"] == "" {
			t.Error("expected document to still be accessible after completion")
		}
	})

	t.Run("completed topics cannot have new interview messages added without reopening", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		topicID := "topic-complete-003"
		store.Create(topicID, "Completed Topic", "Cannot add messages")

		// Mark as completed
		payload := map[string]string{"status": "completed"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/api/topics/"+topicID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Try to add a message to completed topic
		msgPayload := map[string]string{"content": "This should fail"}
		msgBody, _ := json.Marshal(msgPayload)
		msgReq := httptest.NewRequest("POST", "/api/topics/"+topicID+"/messages", bytes.NewReader(msgBody))
		msgReq.Header.Set("Content-Type", "application/json")
		msgResp, err := app.Test(msgReq)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer msgResp.Body.Close()

		if msgResp.StatusCode != http.StatusConflict {
			t.Errorf("expected status 409 Conflict for completed topic, got %d", msgResp.StatusCode)
		}
	})

	t.Run("user can mark a topic complete at any time even if no interview has started", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		topicID := "topic-complete-004"
		store.Create(topicID, "Empty Topic", "No interview started")

		// Mark as completed without any messages
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

	// E3-S11: User manually ends the interview
	t.Run("conclusion is a deliberate user-driven action", func(t *testing.T) {
		suite := newSuiteWithMock(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Response."}}]}`))
		})
		defer suite.Cleanup(t)
		app := suite.App
		store := suite.Store

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
