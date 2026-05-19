package api

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

// E4-S8: Document viewer flashes to indicate when the document changed

func TestDocumentFlashIndicator(t *testing.T) {
	t.Run("topic page includes the document flash indicator element", func(t *testing.T) {
		suite := newSuite(t)
		app := suite.App

		// Serve the topic page
		req := httptest.NewRequest("GET", "/topics/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Check for the flash indicator element
		if !strings.Contains(bodyStr, "doc-flash-indicator") {
			t.Error("expected topic page to include doc-flash-indicator element")
		}
		if !strings.Contains(bodyStr, "Document Updated") {
			t.Error("expected topic page to include 'Document Updated' flash text")
		}
	})

	t.Run("topic page includes the flash animation CSS", func(t *testing.T) {
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

		// Check for CSS animation
		if !strings.Contains(bodyStr, "doc-flash") {
			t.Error("expected topic page to include doc-flash CSS animation")
		}
		if !strings.Contains(bodyStr, "doc-changed") {
			t.Error("expected topic page to include doc-changed CSS class")
		}
		if !strings.Contains(bodyStr, "flash-indicator-fade") {
			t.Error("expected topic page to include flash-indicator-fade animation")
		}
	})

	t.Run("topic page includes the flashDocumentPanel JavaScript function", func(t *testing.T) {
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

		// Check for JavaScript function
		if !strings.Contains(bodyStr, "flashDocumentPanel") {
			t.Error("expected topic page to include flashDocumentPanel function")
		}
		if !strings.Contains(bodyStr, "previousDocument") {
			t.Error("expected topic page to include previousDocument variable for change tracking")
		}
	})

	t.Run("doc-panel has an id for JavaScript targeting", func(t *testing.T) {
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

		// Check for doc-panel id
		if !strings.Contains(bodyStr, `id="doc-panel"`) {
			t.Error("expected doc-panel element to have id='doc-panel'")
		}
	})
}
