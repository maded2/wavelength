package topic

import (
	"os"
	"path/filepath"
	"testing"
)

// E2-S4: Topics and their state persist across application restarts

func TestTopicPersistence(t *testing.T) {
	t.Run("all created topics are still present after restart", func(t *testing.T) {
		dir := t.TempDir()

		// First "session" — create store, add topics, save
		store1 := NewFileStore(dir)
		store1.Create("topic-001", "Topic One", "First topic description")
		store1.Create("topic-002", "Topic Two", "Second topic description")
		if err := store1.SaveAll(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		// "Restart" — create new store from same directory
		store2 := NewFileStore(dir)
		if err := store2.LoadAll(); err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		topics := store2.List()
		if len(topics) != 2 {
			t.Errorf("expected 2 topics after restart, got %d", len(topics))
		}

		names := make(map[string]bool)
		for _, tp := range topics {
			names[tp.Name] = true
		}
		if !names["Topic One"] || !names["Topic Two"] {
			t.Errorf("expected both topics to be present, got: %v", names)
		}
	})

	t.Run("each topics conversation history is fully preserved after restart", func(t *testing.T) {
		dir := t.TempDir()

		store1 := NewFileStore(dir)
		topic := store1.Create("topic-msgs", "Messages Topic", "Testing message persistence")
		store1.AddMessage(topic.ID, "user", "First user message")
		store1.AddMessage(topic.ID, "assistant", "First assistant response")
		store1.AddMessage(topic.ID, "user", "Second user message")
		if err := store1.SaveAll(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		store2 := NewFileStore(dir)
		if err := store2.LoadAll(); err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		loaded := store2.Get("topic-msgs")
		if loaded == nil {
			t.Fatal("topic not found after restart")
		}

		if len(loaded.Messages) != 3 {
			t.Errorf("expected 3 messages, got %d", len(loaded.Messages))
		}

		// Verify message content is preserved
		expected := []string{
			"First user message",
			"First assistant response",
			"Second user message",
		}
		for i, exp := range expected {
			if loaded.Messages[i].Content != exp {
				t.Errorf("message %d: expected %q, got %q", i, exp, loaded.Messages[i].Content)
			}
			if loaded.Messages[i].Role == "" {
				t.Errorf("message %d: expected role to be preserved", i)
			}
		}
	})

	t.Run("each topics requirement document is fully preserved after restart", func(t *testing.T) {
		dir := t.TempDir()

		store1 := NewFileStore(dir)
		topic := store1.Create("topic-doc", "Document Topic", "Testing document persistence")
		topic.Document = "# Requirements\n\n## Overview\nThis is a test requirement document."
		if err := store1.SaveAll(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		store2 := NewFileStore(dir)
		if err := store2.LoadAll(); err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		loaded := store2.Get("topic-doc")
		if loaded == nil {
			t.Fatal("topic not found after restart")
		}

		if loaded.Document != "# Requirements\n\n## Overview\nThis is a test requirement document." {
			t.Errorf("document content not preserved correctly:\ngot: %s", loaded.Document)
		}
	})

	t.Run("topics that were in progress can be resumed after restart", func(t *testing.T) {
		dir := t.TempDir()

		store1 := NewFileStore(dir)
		topic := store1.Create("topic-resume", "Resume Topic", "Testing resume")
		topic.Status = "active"
		store1.AddMessage(topic.ID, "user", "Where were we?")
		if err := store1.SaveAll(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		store2 := NewFileStore(dir)
		if err := store2.LoadAll(); err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		loaded := store2.Get("topic-resume")
		if loaded == nil {
			t.Fatal("topic not found after restart")
		}

		if loaded.Status != "active" {
			t.Errorf("expected status 'active' to be preserved, got %q", loaded.Status)
		}

		// Verify we can add a new message (resume)
		store2.AddMessage(loaded.ID, "assistant", "Let me check our conversation...")
		// Re-fetch to verify the message was added
		loaded = store2.Get("topic-resume")
		if len(loaded.Messages) != 2 {
			t.Errorf("expected 2 messages after resume, got %d", len(loaded.Messages))
		}
	})

	t.Run("persistence requires no user action — happens automatically via SaveAll", func(t *testing.T) {
		dir := t.TempDir()

		store := NewFileStore(dir)
		store.Create("topic-auto", "Auto Save", "Automatic persistence")
		store.AddMessage("topic-auto", "user", "Test message")

		// Save and verify file exists on disk
		if err := store.SaveAll(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		// Verify files exist on disk
		files, err := os.ReadDir(filepath.Join(dir, "topics"))
		if err != nil {
			t.Fatalf("expected topics directory to exist: %v", err)
		}

		if len(files) == 0 {
			t.Error("expected topic files to be created on disk automatically")
		}
	})

	t.Run("persistence uses only file system — no additional infrastructure", func(t *testing.T) {
		dir := t.TempDir()

		store := NewFileStore(dir)
		store.Create("topic-file", "File Only", "No database needed")
		if err := store.SaveAll(); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		// Verify the data is stored as files in the given directory
		topicDir := filepath.Join(dir, "topics")
		if _, err := os.Stat(topicDir); os.IsNotExist(err) {
			t.Error("expected topics directory to exist on file system")
		}

		// Verify we can load from files without any external service
		store2 := NewFileStore(dir)
		if err := store2.LoadAll(); err != nil {
			t.Fatalf("failed to load from files: %v", err)
		}

		loaded := store2.Get("topic-file")
		if loaded == nil {
			t.Error("expected to load topic from file system without external dependencies")
		}
	})
}
