package topic

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// E4-S5: Requirement document persists across application restarts

func TestDocumentPersistence(t *testing.T) {
	t.Run("after restart each topics requirement document contains all content from before", func(t *testing.T) {
		dir := t.TempDir()

		store1 := NewFileStore(dir)
		topic := store1.Create("topic-doc-persist-001", "Persist Doc", "Testing doc persistence")
		topic.Document = "# Requirements\n\n## Overview\n\nThis is a detailed requirement document.\n\n## Users\n\n- Admin\n- Customer\n- Manager"
		if err := store1.SaveAll(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		store2 := NewFileStore(dir)
		if err := store2.LoadAll(); err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		loaded := store2.Get("topic-doc-persist-001")
		if loaded == nil {
			t.Fatal("topic not found after restart")
		}

		if loaded.Document != "# Requirements\n\n## Overview\n\nThis is a detailed requirement document.\n\n## Users\n\n- Admin\n- Customer\n- Manager" {
			t.Errorf("document content not preserved:\nexpected: # Requirements\\n\\n## Overview\\n\\nThis is a detailed requirement document.\\n\\n## Users\\n\\n- Admin\\n- Customer\\n- Manager\n\ngot: %s", loaded.Document)
		}
	})

	t.Run("the documents structure and formatting are preserved exactly", func(t *testing.T) {
		dir := t.TempDir()

		store1 := NewFileStore(dir)
		topic := store1.Create("topic-doc-persist-002", "Format Test", "Testing format preservation")
		topic.Document = "# Title\n\n## Section 1\n\n- Item 1\n- Item 2\n\n### Subsection\n\n> Quote here\n\n```code block```"
		if err := store1.SaveAll(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		store2 := NewFileStore(dir)
		if err := store2.LoadAll(); err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		loaded := store2.Get("topic-doc-persist-002")
		if loaded == nil {
			t.Fatal("topic not found after restart")
		}

		if loaded.Document != "# Title\n\n## Section 1\n\n- Item 1\n- Item 2\n\n### Subsection\n\n> Quote here\n\n```code block```" {
			t.Errorf("document formatting not preserved:\ngot: %s", loaded.Document)
		}
	})

	t.Run("no data corruption or truncation occurs during persistence", func(t *testing.T) {
		dir := t.TempDir()

		store1 := NewFileStore(dir)
		topic := store1.Create("topic-doc-persist-003", "Large Doc", "Testing large document")

		// Create a large document
		var doc string
		for i := 0; i < 100; i++ {
			doc += fmt.Sprintf("## Section %d\n\nThis is the content for section %d with some details.\n\n", i, i)
		}
		topic.Document = "# Large Document\n\n" + doc

		if err := store1.SaveAll(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		store2 := NewFileStore(dir)
		if err := store2.LoadAll(); err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		loaded := store2.Get("topic-doc-persist-003")
		if loaded == nil {
			t.Fatal("topic not found after restart")
		}

		if len(loaded.Document) != len(topic.Document) {
			t.Errorf("document length changed: expected %d, got %d", len(topic.Document), len(loaded.Document))
		}

		if loaded.Document != topic.Document {
			t.Error("document content was corrupted or truncated")
		}
	})

	t.Run("the document is available immediately upon restart with no user action required", func(t *testing.T) {
		dir := t.TempDir()

		store1 := NewFileStore(dir)
		topic := store1.Create("topic-doc-persist-004", "Instant Access", "Testing instant access")
		topic.Document = "# Ready Document\n\nContent is ready."
		if err := store1.SaveAll(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		// Simulate restart — load and immediately access
		store2 := NewFileStore(dir)
		if err := store2.LoadAll(); err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		// No additional save or recovery needed
		loaded := store2.Get("topic-doc-persist-004")
		if loaded == nil {
			t.Fatal("topic not found immediately after restart")
		}

		if loaded.Document != "# Ready Document\n\nContent is ready." {
			t.Errorf("document not immediately available, got: %s", loaded.Document)
		}
	})

	t.Run("persistence behavior is automatic and requires no user awareness", func(t *testing.T) {
		dir := t.TempDir()

		store := NewFileStore(dir)
		topic := store.Create("topic-doc-persist-005", "Auto Persist", "Testing auto persist")
		topic.Document = "# Auto Document\n\nPersisted automatically."

		// Save and verify directory exists on disk automatically
		if err := store.SaveAll(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		// Verify the topic directory and document.md exist on disk
		topicDir := filepath.Join(dir, "topics", "topic-doc-persist-005")
		if _, err := os.Stat(topicDir); os.IsNotExist(err) {
			t.Error("expected topic directory to exist on disk automatically")
		}
		docFile := filepath.Join(topicDir, "document.md")
		if _, err := os.Stat(docFile); os.IsNotExist(err) {
			t.Error("expected document.md to exist on disk automatically")
		}

		// Verify we can load without any user action
		store2 := NewFileStore(dir)
		if err := store2.LoadAll(); err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		loaded := store2.Get("topic-doc-persist-005")
		if loaded == nil {
			t.Error("expected topic to be loaded automatically")
		}
		if loaded.Document != "# Auto Document\n\nPersisted automatically." {
			t.Errorf("document not persisted automatically, got: %s", loaded.Document)
		}
	})
}
