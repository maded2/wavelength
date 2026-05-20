package api

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

// E4-S8: Document viewer flashes to indicate when the document changed

func TestDocumentFlashIndicator(t *testing.T) {
	t.Run("topic page provides visual feedback when the document changes", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App

		req := httptest.NewRequest("GET", "/topics/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// The page should include a flash indicator element that the user can see
		if !strings.Contains(bodyStr, "doc-flash-indicator") {
			t.Error("expected topic page to include document change indicator element")
		}
		// The indicator should display user-facing text about document updates
		if !strings.Contains(bodyStr, "Document Updated") {
			t.Error("expected topic page to include 'Document Updated' user-facing text")
		}
	})

	t.Run("topic page includes styling for the flash indicator", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App

		req := httptest.NewRequest("GET", "/topics/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// The page should include CSS for the flash animation effect
		if !strings.Contains(bodyStr, "doc-flash") {
			t.Error("expected topic page to include flash animation CSS")
		}
		// The page should include CSS for the fade-out effect
		if !strings.Contains(bodyStr, "flash-indicator-fade") {
			t.Error("expected topic page to include fade-out CSS animation")
		}
	})

	t.Run("topic page includes JavaScript to detect document changes", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App

		req := httptest.NewRequest("GET", "/topics/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// The page should include a function that triggers the flash on document change
		if !strings.Contains(bodyStr, "flashDocumentPanel") {
			t.Error("expected topic page to include document change flash function")
		}
		// The page should track the previous document content to detect changes
		if !strings.Contains(bodyStr, "previousDocument") {
			t.Error("expected topic page to include document change tracking variable")
		}
	})

	t.Run("document panel has a targetable element for flash updates", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App

		req := httptest.NewRequest("GET", "/topics/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// The document panel should have an id so JavaScript can target it
		if !strings.Contains(bodyStr, `id="doc-panel"`) {
			t.Error("expected document panel to have a targetable id")
		}
	})
}
