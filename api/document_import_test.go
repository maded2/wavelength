package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// E4-S7: User provides a pre-existing document as a starting point

func TestPreExistingDocument(t *testing.T) {
	t.Run("user can provide a pre-existing document when creating a topic", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App

		payload := map[string]string{
			"name":        "Imported Project",
			"description": "A project with existing requirements",
			"document":    "# Existing Requirements\n\n## Overview\n\nAlready written content.",
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

		// Verify the document was set
		doc, ok := created["document"].(string)
		if !ok {
			t.Fatal("expected document field in created topic")
		}
		if doc != "# Existing Requirements\n\n## Overview\n\nAlready written content." {
			t.Errorf("expected pre-existing document to be set, got: %s", doc)
		}
	})

	t.Run("the provided document becomes the starting point for the topics requirement document", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App

		payload := map[string]string{
			"name":        "Seed Doc",
			"description": "Seeded with existing doc",
			"document":    "# Seed Document\n\n## Section 1\n\nPre-existing content.",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Verify via detail view
		listReq := httptest.NewRequest("GET", "/api/topics", nil)
		listResp, _ := app.Test(listReq)
		var topics []map[string]interface{}
		json.NewDecoder(listResp.Body).Decode(&topics)
		listResp.Body.Close()

		if len(topics) != 1 {
			t.Fatalf("expected 1 topic, got %d", len(topics))
		}

		topicID := topics[0]["id"].(string)
		detailReq := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp, _ := app.Test(detailReq)
		var detail map[string]interface{}
		json.NewDecoder(detailResp.Body).Decode(&detail)
		detailResp.Body.Close()

		doc := detail["document"].(string)
		if doc != "# Seed Document\n\n## Section 1\n\nPre-existing content." {
			t.Errorf("expected seeded document, got: %s", doc)
		}
	})

	t.Run("providing a pre-existing document is optional", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App

		// Create topic without document field
		payload := map[string]string{
			"name":        "No Doc Topic",
			"description": "Starting from scratch",
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
			t.Errorf("expected status 201 Created without document, got %d", resp.StatusCode)
		}

		var created map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		// Document should be the blank template (not empty)
		doc := created["document"].(string)
		if doc == "" {
			t.Error("expected blank document template when no document provided, got empty string")
		}
		if !strings.Contains(doc, "# Requirements: No Doc Topic") {
			t.Errorf("expected document template with topic name, got: %s", doc)
		}
	})

	t.Run("non-markdown content is accepted as plain text", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App

		// Plain text without markdown formatting
		payload := map[string]string{
			"name":        "Plain Text Doc",
			"description": "Plain text import",
			"document":    "This is just plain text without any markdown formatting. It should still be accepted.",
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
			t.Errorf("expected status 201 for plain text document, got %d", resp.StatusCode)
		}

		var created map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
			t.Fatalf("expected JSON response, got: %v", err)
		}

		doc := created["document"].(string)
		if doc != "This is just plain text without any markdown formatting. It should still be accepted." {
			t.Errorf("expected plain text document to be accepted, got: %s", doc)
		}
	})

	t.Run("the pre-existing document is preserved and not discarded", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App

		payload := map[string]string{
			"name":        "Preserve Doc",
			"description": "Must preserve this doc",
			"document":    "# Must Not Be Lost\n\n## Important Section\n\nCritical content that must be preserved.",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/topics", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)
		resp.Body.Close()

		// Get topic ID
		listReq := httptest.NewRequest("GET", "/api/topics", nil)
		listResp, _ := app.Test(listReq)
		var topics []map[string]interface{}
		json.NewDecoder(listResp.Body).Decode(&topics)
		listResp.Body.Close()

		topicID := topics[0]["id"].(string)

		// Verify document is still there
		detailReq := httptest.NewRequest("GET", "/api/topics/"+topicID, nil)
		detailResp, _ := app.Test(detailReq)
		var detail map[string]interface{}
		json.NewDecoder(detailResp.Body).Decode(&detail)
		detailResp.Body.Close()

		doc := detail["document"].(string)
		if doc != "# Must Not Be Lost\n\n## Important Section\n\nCritical content that must be preserved." {
			t.Errorf("expected pre-existing document to be preserved, got: %s", doc)
		}
	})
}
