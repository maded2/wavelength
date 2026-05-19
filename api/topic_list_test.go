package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// E2-S2: User views list of all topics

func TestTopicList(t *testing.T) {
	t.Run("the topic list displays every topic that has been created", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		// Create 3 topics
		for i := 1; i <= 3; i++ {
			store.Create("topic-0000000000000000000"+string(rune('0'+i)), "Topic "+string(rune('0'+i)), "Description "+string(rune('0'+i)))
		}

		req := httptest.NewRequest("GET", "/api/topics", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var topics []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&topics); err != nil {
			t.Fatalf("expected JSON array, got: %v", err)
		}

		if len(topics) != 3 {
			t.Errorf("expected 3 topics in list, got %d", len(topics))
		}
	})

	t.Run("each topic entry shows the topic name, status, and last updated time", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		store.Create("topic-0000000000000000001", "My Topic", "A test topic")

		req := httptest.NewRequest("GET", "/api/topics", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var topics []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&topics); err != nil {
			t.Fatalf("expected JSON array, got: %v", err)
		}

		if len(topics) != 1 {
			t.Fatalf("expected 1 topic, got %d", len(topics))
		}

		entry := topics[0]

		// Check name
		if entry["name"] == "" || entry["name"] == nil {
			t.Error("expected topic entry to have a name")
		}

		// Check status
		status, ok := entry["status"].(string)
		if !ok || status == "" {
			t.Error("expected topic entry to have a status")
		}

		// Check updated_at
		if entry["updated_at"] == "" || entry["updated_at"] == nil {
			t.Error("expected topic entry to have an updated_at timestamp")
		}
	})

	t.Run("the list distinguishes between topics that have started interviews and those that have not", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		// Create a topic that hasn't started
		store.Create("topic-not-started", "Not Started", "No interview yet")

		// Create a topic that has started (has messages)
		activeTopic := store.Create("topic-active", "Active", "Interview in progress")
		activeTopic.Status = "active"

		req := httptest.NewRequest("GET", "/api/topics", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var topics []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&topics); err != nil {
			t.Fatalf("expected JSON array, got: %v", err)
		}

		if len(topics) != 2 {
			t.Fatalf("expected 2 topics, got %d", len(topics))
		}

		// Verify statuses are different
		statuses := make(map[string]bool)
		for _, tp := range topics {
			statuses[tp["status"].(string)] = true
		}

		if !statuses["not_started"] || !statuses["active"] {
			t.Errorf("expected both 'not_started' and 'active' statuses, got: %v", statuses)
		}
	})

	t.Run("the list is ordered with the most recently updated topics first", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App
		store := suite.Store

		// Create topics with different update times
		oldTopic := store.Create("topic-old", "Old Topic", "Created first")
		oldTopic.UpdatedAt = time.Now().Add(-24 * time.Hour)

		newTopic := store.Create("topic-new", "New Topic", "Created last")
		newTopic.UpdatedAt = time.Now()

		req := httptest.NewRequest("GET", "/api/topics", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var topics []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&topics); err != nil {
			t.Fatalf("expected JSON array, got: %v", err)
		}

		if len(topics) != 2 {
			t.Fatalf("expected 2 topics, got %d", len(topics))
		}

		// First topic should be the most recently updated one
		firstName := topics[0]["name"].(string)
		if firstName != "New Topic" {
			t.Errorf("expected first topic to be 'New Topic' (most recently updated), got %q", firstName)
		}
	})

	t.Run("if there are no topics the list shows a helpful message", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App

		req := httptest.NewRequest("GET", "/api/topics", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}
		bodyStr := string(body)

		// Empty list should be an empty array, and the landing page HTML
		// should contain a helpful message about creating topics
		if bodyStr != "[]" {
			t.Logf("response body: %s", bodyStr)
		}

		// Also check the landing page for the empty state message
		landingReq := httptest.NewRequest("GET", "/", nil)
		landingResp, err := app.Test(landingReq)
		if err != nil {
			t.Fatalf("failed to get landing page: %v", err)
		}
		defer landingResp.Body.Close()

		landingBody, err := io.ReadAll(landingResp.Body)
		if err != nil {
			t.Fatalf("failed to read landing page: %v", err)
		}
		landingStr := string(landingBody)

		// Landing page should guide users to create their first topic
		lowerLanding := strings.ToLower(landingStr)
		hasGuidance := strings.Contains(lowerLanding, "create") &&
			(strings.Contains(lowerLanding, "topic") || strings.Contains(lowerLanding, "first"))
		if !hasGuidance {
			t.Error("expected landing page to guide users to create their first topic when no topics exist")
		}
	})
}
