package api

import (
	"strings"
	"testing"

	"wavelength/internal/topic"
)

func TestBuildConversationContext(t *testing.T) {
	t.Run("short conversation includes all messages verbatim", func(t *testing.T) {
		topic := &topic.Topic{
			ID:          "test",
			Name:        "Test Topic",
			Description: "A test topic",
			Messages: []topic.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there"},
				{Role: "user", Content: "I need a login system"},
			},
			Document: "# Requirements: Test Topic\n\n## Overview\n\nA test topic",
		}

		prompt := buildConversationContext(topic, "I also need password reset")

		// All messages should appear in the prompt
		if !strings.Contains(prompt, "Hello") || !strings.Contains(prompt, "Hi there") || !strings.Contains(prompt, "I need a login system") {
			t.Errorf("expected all messages in prompt, got:\n%s", prompt)
		}

		// Should NOT have summary markers since it's short
		if strings.Contains(prompt, "Conversation summary") {
			t.Errorf("expected no summary in short conversation, got:\n%s", prompt)
		}
	})

	t.Run("long conversation triggers summarization", func(t *testing.T) {
		messages := make([]topic.Message, 0, 200)
		// Create 200 messages with substantial content to exceed the context limit
		for i := 0; i < 200; i++ {
			if i%2 == 0 {
				messages = append(messages, topic.Message{
					Role: "user",
					Content: "User point number " + string(rune('0'+i%10)) + ": This is a detailed requirement about feature " + string(rune('A'+i%26)) + ". The system should support multiple users with different roles and permissions. Each user should be able to create, read, update, and delete resources within their authorized scope. The system must handle concurrent access and maintain data integrity. Additionally, the system should provide audit logging for all user actions and support export of data in multiple formats including CSV and PDF.",
				})
			} else {
				messages = append(messages, topic.Message{
					Role: "assistant",
					Content: "Assistant insight number " + string(rune('0'+i%10)) + ": I understand. Let me clarify some details about this requirement. Could you specify what types of roles you envision? Also, what are the expected concurrency levels? Regarding audit logging, do you need real-time monitoring or batch processing would suffice? For data exports, are there specific formatting requirements or compliance standards we need to consider?",
				})
			}
		}

		topic := &topic.Topic{
			ID:          "test-long",
			Name:        "Long Conversation",
			Description: "A topic with a very long conversation",
			Messages:    messages,
			Document:    "# Requirements: Long Conversation\n\n## Overview\n\nA topic with a very long conversation",
		}

		prompt := buildConversationContext(topic, "Final message in the conversation")

		// Should have summary markers
		if !strings.Contains(prompt, "Conversation summary") {
			t.Errorf("expected summary markers in long conversation, got:\n%s", prompt)
		}

		// Should have recent conversation section
		if !strings.Contains(prompt, "Recent conversation") {
			t.Errorf("expected recent conversation section, got:\n%s", prompt)
		}

		// Should include topic name and description
		if !strings.Contains(prompt, "Long Conversation") || !strings.Contains(prompt, "A topic with a very long conversation") {
			t.Errorf("expected topic name and description, got:\n%s", prompt)
		}

		// Should include current document
		if !strings.Contains(prompt, "Current requirement document") {
			t.Errorf("expected current requirement document, got:\n%s", prompt)
		}

		// Should include user's latest message
		if !strings.Contains(prompt, "Final message in the conversation") {
			t.Errorf("expected user's latest message, got:\n%s", prompt)
		}
	})

	t.Run("document is truncated when very long", func(t *testing.T) {
		// Create a very long document
		longDoc := "# Requirements\n\n"
		for i := 0; i < 500; i++ {
			longDoc += "Section " + string(rune('0'+i%10)) + ": This is a detailed section with lots of content about the system requirements.\n"
		}

		topic := &topic.Topic{
			ID:          "test-doc",
			Name:        "Long Doc",
			Description: "A topic with a long document",
			Messages: []topic.Message{
				{Role: "user", Content: "Hello"},
			},
			Document: longDoc,
		}

		prompt := buildConversationContext(topic, "Test message")

		// Document should be truncated
		if !strings.Contains(prompt, "...(truncated for context)") {
			t.Errorf("expected truncated document marker, got:\n%s", prompt)
		}
	})

	t.Run("empty messages handled gracefully", func(t *testing.T) {
		topic := &topic.Topic{
			ID:          "test-empty",
			Name:        "Empty Topic",
			Description: "An empty topic",
			Messages:    []topic.Message{},
			Document:    "",
		}

		prompt := buildConversationContext(topic, "First message")

		if !strings.Contains(prompt, "Empty Topic") {
			t.Errorf("expected topic name, got:\n%s", prompt)
		}

		if !strings.Contains(prompt, "First message") {
			t.Errorf("expected user message, got:\n%s", prompt)
		}
	})

	t.Run("no description handled gracefully", func(t *testing.T) {
		topic := &topic.Topic{
			ID:          "test-no-desc",
			Name:        "No Desc Topic",
			Description: "",
			Messages:    []topic.Message{},
			Document:    "",
		}

		prompt := buildConversationContext(topic, "Hello")

		if !strings.Contains(prompt, "(no description provided)") {
			t.Errorf("expected '(no description provided)' placeholder, got:\n%s", prompt)
		}
	})
}

func TestSummarizeMessages(t *testing.T) {
	t.Run("empty messages returns placeholder", func(t *testing.T) {
		summary := summarizeMessages([]topic.Message{})
		if !strings.Contains(summary, "no prior conversation") {
			t.Errorf("expected placeholder for empty messages, got:\n%s", summary)
		}
	})

	t.Run("summarizes user and assistant messages separately", func(t *testing.T) {
		messages := []topic.Message{
			{Role: "user", Content: "I need user authentication"},
			{Role: "assistant", Content: "What type of authentication? SSO, OAuth, or basic?"},
			{Role: "user", Content: "SSO with SAML"},
			{Role: "assistant", Content: "Got it. Which IdP will you use?"},
		}

		summary := summarizeMessages(messages)

		if !strings.Contains(summary, "Key points from stakeholder") {
			t.Errorf("expected user points section, got:\n%s", summary)
		}

		if !strings.Contains(summary, "Key insights and questions from analyst") {
			t.Errorf("expected assistant insights section, got:\n%s", summary)
		}

		if !strings.Contains(summary, "user authentication") && !strings.Contains(summary, "I need user authentication") {
			t.Errorf("expected user content in summary, got:\n%s", summary)
		}
	})

	t.Run("truncates very long messages", func(t *testing.T) {
		longContent := "This is a very long message. "
		for i := 0; i < 100; i++ {
			longContent += "It contains lots of details about the system requirements and how they should work. "
		}

		messages := []topic.Message{
			{Role: "user", Content: longContent},
		}

		summary := summarizeMessages(messages)

		// Summary should be much shorter than original
		if len(summary) >= len(longContent) {
			t.Errorf("expected summary to be shorter than original, got %d vs %d", len(summary), len(longContent))
		}
	})

	t.Run("skips empty messages", func(t *testing.T) {
		messages := []topic.Message{
			{Role: "user", Content: ""},
			{Role: "assistant", Content: "   "},
			{Role: "user", Content: "Valid message"},
		}

		summary := summarizeMessages(messages)

		// Should only contain the valid message
		if !strings.Contains(summary, "Valid message") {
			t.Errorf("expected valid message in summary, got:\n%s", summary)
		}
	})
}


