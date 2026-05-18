package interview

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// mockLLM implements LLMClient for testing.
type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Call(_ context.Context, _ []llm.Message) (string, error) {
	return m.response, m.err
}

func (m *mockLLM) PersonaPrompt() string {
	return "You are a business analyst."
}

// --- ExtractDocument Tests ---

func TestExtractDocument(t *testing.T) {
	tests := []struct {
		name             string
		response         string
		wantConvContains []string
		wantConvNot      []string
		wantDocContains  []string
		wantDocEmpty     bool
	}{
		{
			name:             "no delimiters returns full response as conversational",
			response:         "Sure, let me ask you about the user roles.",
			wantConvContains: []string{"Sure, let me ask"},
			wantDocEmpty:     true,
		},
		{
			name:             "document between delimiters is extracted",
			response:         "Thanks!\n\n=== REQUIREMENT DOCUMENT ===\n# Requirements\n=== END REQUIREMENT DOCUMENT ===\n\nFollow up?",
			wantConvContains: []string{"Thanks!", "Follow up?"},
			wantConvNot:      []string{"# Requirements"},
			wantDocContains:  []string{"# Requirements"},
		},
		{
			name:            "document only returns empty conversational",
			response:        "\n=== REQUIREMENT DOCUMENT ===\n# Doc\n=== END REQUIREMENT DOCUMENT ===\n",
			wantDocContains: []string{"# Doc"},
		},
		{
			name:             "single delimiter treated as conversational",
			response:         "=== REQUIREMENT DOCUMENT ===\n# No closing",
			wantConvContains: []string{"# No closing"},
			wantDocEmpty:     true,
		},
		{
			name:             "markdown horizontal rules do not trigger false extraction",
			response:         "---\n## Key Points\n---",
			wantConvContains: []string{"Key Points"},
			wantDocEmpty:     true,
		},
		{
			name: "complex document with tables is preserved",
			response: `=== REQUIREMENT DOCUMENT ===
### Living Requirements
| Role | Access |
|------|--------|
| Admin | Full |
=== END REQUIREMENT DOCUMENT ===`,
			wantDocContains: []string{"Living", "| Admin | Full |"},
			wantConvNot:     []string{"Living"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv, doc := ExtractDocument(tt.response)

			if tt.wantDocEmpty && doc != "" {
				t.Errorf("expected empty document, got: %q", doc)
			}
			for _, s := range tt.wantDocContains {
				if !strings.Contains(doc, s) {
					t.Errorf("expected document to contain %q, got: %s", s, doc)
				}
			}
			for _, s := range tt.wantConvContains {
				if !strings.Contains(conv, s) {
					t.Errorf("expected conversational to contain %q, got: %s", s, conv)
				}
			}
			for _, s := range tt.wantConvNot {
				if strings.Contains(conv, s) {
					t.Errorf("expected conversational to NOT contain %q, got: %s", s, conv)
				}
			}
		})
	}
}

// --- BuildConversationContext Tests ---

func TestBuildConversationContext(t *testing.T) {
	t.Run("short conversation includes all messages verbatim", func(t *testing.T) {
		tp := &topic.Topic{
			Name:        "Test",
			Description: "A test",
			Messages: []topic.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there"},
			},
			Document: "# Requirements\n\n## Overview\n\nA test",
		}

		prompt := BuildConversationContext(tp, "Need password reset")

		if !strings.Contains(prompt, "Hello") {
			t.Error("expected 'Hello' in prompt")
		}
		if !strings.Contains(prompt, "Hi there") {
			t.Error("expected 'Hi there' in prompt")
		}
		if strings.Contains(prompt, "Conversation summary") {
			t.Error("short conversation should NOT have summary markers")
		}
	})

	t.Run("no document instructs LLM to create one", func(t *testing.T) {
		tp := &topic.Topic{
			Name:        "Fresh",
			Description: "Fresh topic",
			Messages:    []topic.Message{},
			Document:    "",
		}

		prompt := BuildConversationContext(tp, "Let's start")

		if !strings.Contains(prompt, "No requirement document exists yet") {
			t.Error("expected instruction to create initial document")
		}
		if !strings.Contains(prompt, "=== REQUIREMENT DOCUMENT ===") {
			t.Error("expected delimiter instructions in prompt")
		}
	})

	t.Run("long conversation triggers summarization", func(t *testing.T) {
		tp := &topic.Topic{
			Name:        "Long",
			Description: "Long topic",
			Messages:    make([]topic.Message, 100),
			Document:    "# Requirements",
		}
		for i := range tp.Messages {
			tp.Messages[i] = topic.Message{Role: "user", Content: strings.Repeat("x", 800)}
		}

		prompt := BuildConversationContext(tp, "Final message")

		if !strings.Contains(prompt, "Conversation summary") {
			t.Error("expected summary markers for long conversation")
		}
	})
}

// --- SummarizeMessages Tests ---

func TestSummarizeMessages(t *testing.T) {
	t.Run("empty messages returns placeholder", func(t *testing.T) {
		summary := SummarizeMessages(nil)
		if !strings.Contains(summary, "no prior conversation") {
			t.Errorf("expected placeholder, got: %s", summary)
		}
	})

	t.Run("separates user and assistant points", func(t *testing.T) {
		msgs := []topic.Message{
			{Role: "user", Content: "I need auth"},
			{Role: "assistant", Content: "What kind of auth?"},
		}
		summary := SummarizeMessages(msgs)
		if !strings.Contains(summary, "stakeholder") {
			t.Error("expected 'stakeholder' section")
		}
		if !strings.Contains(summary, "analyst") {
			t.Error("expected 'analyst' section")
		}
	})

	t.Run("truncates very long messages", func(t *testing.T) {
		msgs := []topic.Message{
			{Role: "user", Content: strings.Repeat("word ", 500)},
		}
		summary := SummarizeMessages(msgs)
		if !strings.HasSuffix(strings.TrimSpace(summary), "...") {
			t.Error("expected truncated summary to end with '...'")
		}
	})
}

// --- HandleMessage Tests ---

func TestHandleMessage(t *testing.T) {
	t.Run("saves user and assistant messages", func(t *testing.T) {
		store := topic.NewStore()
		store.Create("t1", "Test", "Description")
		client := &mockLLM{response: "Assistant reply"}
		svc := New(store, client)

		conv, docUpdated, err := svc.HandleMessage(context.Background(), "t1", "Hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if docUpdated {
			t.Error("expected no document update for plain response")
		}
		if conv != "Assistant reply" {
			t.Errorf("expected 'Assistant reply', got: %q", conv)
		}

		loaded := store.Get("t1")
		if len(loaded.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(loaded.Messages))
		}
	})

	t.Run("extracts and saves embedded document", func(t *testing.T) {
		store := topic.NewStore()
		store.Create("t2", "Test", "Description")
		client := &mockLLM{response: "Here's the doc:\n=== REQUIREMENT DOCUMENT ===\n# New Requirements\n=== END REQUIREMENT DOCUMENT ===\nDone!"}
		svc := New(store, client)

		_, docUpdated, err := svc.HandleMessage(context.Background(), "t2", "Start")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !docUpdated {
			t.Error("expected document to be updated")
		}

		loaded := store.Get("t2")
		if !strings.Contains(loaded.Document, "# New Requirements") {
			t.Errorf("expected document to contain '# New Requirements', got: %s", loaded.Document)
		}
	})

	t.Run("returns error when topic not found", func(t *testing.T) {
		store := topic.NewStore()
		client := &mockLLM{}
		svc := New(store, client)

		_, _, err := svc.HandleMessage(context.Background(), "nonexistent", "Hello")
		if err == nil {
			t.Fatal("expected error for nonexistent topic")
		}
	})

	t.Run("returns error when LLM call fails", func(t *testing.T) {
		store := topic.NewStore()
		store.Create("t3", "Test", "Description")
		client := &mockLLM{err: fmt.Errorf("connection refused")}
		svc := New(store, client)

		_, _, err := svc.HandleMessage(context.Background(), "t3", "Hello")
		if err == nil {
			t.Fatal("expected error for failed LLM call")
		}
	})
}

// --- Reevaluate Tests ---

func TestReevaluate(t *testing.T) {
	t.Run("clears conversation history and re-assesses document", func(t *testing.T) {
		store := topic.NewStore()
		topic := store.Create("t4", "Test", "Description")
		topic.Document = "# Existing Doc"
		store.AddMessage("t4", "user", "Old message")
		store.AddMessage("t4", "assistant", "Old response")

		client := &mockLLM{response: "Re-evaluated!\n=== REQUIREMENT DOCUMENT ===\n# Updated Doc\n=== END REQUIREMENT DOCUMENT ==="}
		svc := New(store, client)

		conv, docUpdated, err := svc.Reevaluate(context.Background(), "t4")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !docUpdated {
			t.Error("expected document update after re-evaluation")
		}
		if conv != "Re-evaluated!" {
			t.Errorf("expected 'Re-evaluated!', got: %q", conv)
		}

		loaded := store.Get("t4")
		if len(loaded.Messages) != 1 {
			t.Errorf("expected 1 message after re-eval, got %d", len(loaded.Messages))
		}
	})

	t.Run("returns error when topic not found", func(t *testing.T) {
		store := topic.NewStore()
		client := &mockLLM{}
		svc := New(store, client)

		_, _, err := svc.Reevaluate(context.Background(), "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent topic")
		}
	})
}
