package interview

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// LLMClient defines the subset of llm.Client needed by the interview service.
type LLMClient interface {
	Call(ctx context.Context, messages []llm.Message) (string, error)
	PersonaPrompt() string
}

// Service orchestrates the interview flow between a user, an LLM, and a topic store.
type Service struct {
	store  Store
	client LLMClient
}

// Store defines the subset of topic persistence operations the interview service needs.
type Store interface {
	Get(id string) *topic.Topic
	AddMessage(topicID, role, content string)
	ClearMessages(id string) bool
	SetStatus(id, status string) bool
	SetDocument(id, document string) bool
}

// New creates a new interview service.
func New(store Store, client LLMClient) *Service {
	return &Service{store: store, client: client}
}

// HandleMessage processes a user message for a topic: builds context, calls the LLM,
// extracts any embedded document, and saves the conversation exchange.
// Returns the assistant's conversational response and whether the document was updated.
func (s *Service) HandleMessage(ctx context.Context, topicID, userMessage string) (conversational string, docUpdated bool, err error) {
	topic := s.store.Get(topicID)
	if topic == nil {
		return "", false, fmt.Errorf("topic %q not found", topicID)
	}

	// Save user message
	s.store.AddMessage(topicID, "user", userMessage)

	// Build conversation context with automatic summarization
	prompt := BuildConversationContext(topic, userMessage)

	// Call the LLM
	messages := []llm.Message{
		{Role: "system", Content: s.client.PersonaPrompt()},
		{Role: "user", Content: prompt},
	}

	assistantResponse, err := s.client.Call(ctx, messages)
	if err != nil {
		return "", false, err
	}

	return s.processResponse(topicID, assistantResponse)
}

// Reevaluate clears conversation history and asks the LLM to re-assess the
// current requirement document from scratch.
func (s *Service) Reevaluate(ctx context.Context, topicID string) (conversational string, docUpdated bool, err error) {
	topic := s.store.Get(topicID)
	if topic == nil {
		return "", false, fmt.Errorf("topic %q not found", topicID)
	}

	// Clear all conversation history
	s.store.ClearMessages(topicID)

	// Build re-evaluation prompt
	description := topic.Description
	if description == "" {
		description = "(no description provided)"
	}
	document := topic.Document
	if document == "" {
		document = "(no document yet)"
	}

	reevalPrompt := fmt.Sprintf(
		`You are asked to re-evaluate the requirement document for this topic from scratch.

Topic: %s
High-level requirement: %s

Current requirement document:

%s

Please review the document critically and:
1. Identify any gaps, inconsistencies, or missing sections
2. Suggest improvements to the structure and content
3. Provide an updated version of the document wrapped in the following delimiters if changes are needed:

=== REQUIREMENT DOCUMENT ===
<updated document content>
=== END REQUIREMENT DOCUMENT ===

Remember to maintain the document in markdown format and wrap any updated document in the delimiters above.`,
		topic.Name, description, document,
	)

	// Call LLM with re-evaluation prompt
	messages := []llm.Message{
		{Role: "system", Content: s.client.PersonaPrompt()},
		{Role: "user", Content: reevalPrompt},
	}

	assistantResponse, err := s.client.Call(ctx, messages)
	if err != nil {
		return "", false, err
	}

	return s.processResponse(topicID, assistantResponse)
}

// processResponse extracts any embedded document from the LLM response,
// updates the topic if found, and saves the assistant's message.
func (s *Service) processResponse(topicID string, assistantResponse string) (conversational string, docUpdated bool, err error) {
	conversationalPart, extractedDoc := ExtractDocument(assistantResponse)

	// If a document was extracted, update the topic's requirement document
	if extractedDoc != "" {
		if s.store.SetDocument(topicID, extractedDoc) {
			docUpdated = true
		}
	}

	// Save assistant response
	s.store.AddMessage(topicID, "assistant", conversationalPart)
	return conversationalPart, docUpdated, nil
}

// BuildPrompt constructs the full conversation context for an LLM call.
// It can be used by the streaming handler which needs the prompt for SSE.
func (s *Service) BuildPrompt(topic *topic.Topic, userMessage string) string {
	return BuildConversationContext(topic, userMessage)
}

// --- Private helpers (extracted from api/routes.go) ---

// maxContextChars is the approximate character limit for conversation context.
const maxContextChars = 60000

// maxRecentMessages is how many recent messages to keep verbatim when summarizing.
const maxRecentMessages = 20

// docDelimOpen/Close are the markers for embedded requirement documents in LLM responses.
const docDelimOpen = "=== REQUIREMENT DOCUMENT ==="
const docDelimClose = "=== END REQUIREMENT DOCUMENT ==="

// BuildConversationContext constructs the conversation context string, applying
// summarization when the conversation exceeds maxContextChars.
func BuildConversationContext(t *topic.Topic, userMessage string) string {
	description := t.Description
	if description == "" {
		description = "(no description provided)"
	}

	// Calculate total conversation size
	totalChars := 0
	for _, msg := range t.Messages {
		totalChars += len(msg.Content) + 20
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Topic: %s\nHigh-level requirement: %s\n", t.Name, description))

	// Include uploaded document attachments as reference material
	if len(t.Attachments) > 0 {
		buf.WriteString("\nUploaded reference documents:\n")
		for _, att := range t.Attachments {
			if att.MarkdownContent == "" {
				continue
			}
			buf.WriteString(fmt.Sprintf("\n[Document: %s (%s, %d bytes)]\n", att.Filename, att.Format, att.Size))
			content := att.MarkdownContent
			if len(content) > 8000 {
				content = content[:8000] + "\n...(truncated)"
			}
			buf.WriteString(content + "\n")
		}
		buf.WriteString("\n")
	}

	// Include current requirement document as context
	if t.Document != "" {
		buf.WriteString("\nCurrent requirement document:\n")
		doc := t.Document
		if len(doc) > 4000 {
			doc = doc[:4000] + "\n...(truncated for context)"
		}
		buf.WriteString(doc + "\n")
	} else {
		buf.WriteString("\nNo requirement document exists yet. You should create an initial\n" +
			"requirements document based on what you learn from the stakeholder.\n" +
			"Wrap the complete document in the following delimiters:\n\n" +
			"=== REQUIREMENT DOCUMENT ===\n" +
			"<complete markdown document content here>\n" +
			"=== END REQUIREMENT DOCUMENT ===\n")
	}

	// If conversation fits within limits, include everything
	if totalChars <= maxContextChars || len(t.Messages) <= maxRecentMessages {
		buf.WriteString("\nConversation context:\n")
		for _, msg := range t.Messages {
			fmt.Fprintf(&buf, "%s: %s\n", msg.Role, msg.Content)
		}
		buf.WriteString(fmt.Sprintf("\nUser's latest message: %s\n\nPlease respond as a business analyst conducting requirements gathering.", userMessage))
		return buf.String()
	}

	// Summarize older messages, keep recent ones verbatim
	olderMessages := t.Messages[:len(t.Messages)-maxRecentMessages]
	recentMessages := t.Messages[len(t.Messages)-maxRecentMessages:]

	buf.WriteString("\nConversation summary (earlier exchanges):\n")
	buf.WriteString(SummarizeMessages(olderMessages))

	buf.WriteString("\n\nRecent conversation:\n")
	for _, msg := range recentMessages {
		fmt.Fprintf(&buf, "%s: %s\n", msg.Role, msg.Content)
	}

	buf.WriteString(fmt.Sprintf("\nUser's latest message: %s\n\nPlease respond as a business analyst conducting requirements gathering.", userMessage))
	return buf.String()
}

// SummarizeMessages creates a compact summary of conversation messages.
func SummarizeMessages(messages []topic.Message) string {
	if len(messages) == 0 {
		return "(no prior conversation)"
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Summary of %d earlier exchanges:\n\n", len(messages)))

	var userPoints []string
	var assistantInsights []string

	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if len(content) == 0 {
			continue
		}
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		if msg.Role == "user" {
			userPoints = append(userPoints, content)
		} else {
			assistantInsights = append(assistantInsights, content)
		}
	}

	if len(userPoints) > 0 {
		buf.WriteString("Key points from stakeholder:\n")
		for i, point := range userPoints {
			buf.WriteString(fmt.Sprintf("  %d. %s\n", i+1, point))
		}
		buf.WriteString("\n")
	}

	if len(assistantInsights) > 0 {
		buf.WriteString("Key insights and questions from analyst:\n")
		for i, insight := range assistantInsights {
			buf.WriteString(fmt.Sprintf("  %d. %s\n", i+1, insight))
		}
	}

	return buf.String()
}

// ExtractDocument parses an LLM response to separate the conversational portion from
// any embedded requirements document wrapped in delimiters.
func ExtractDocument(response string) (conversational string, document string) {
	lines := strings.Split(response, "\n")

	openIdx := -1
	closeIdx := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == docDelimOpen {
			if openIdx == -1 {
				openIdx = i
			}
		} else if trimmed == docDelimClose {
			if openIdx != -1 {
				closeIdx = i
				break
			}
		}
	}

	if openIdx == -1 || closeIdx == -1 {
		return strings.TrimSpace(response), ""
	}

	// Reconstruct document content
	docLines := lines[openIdx+1 : closeIdx]
	docContent := strings.TrimSpace(strings.Join(docLines, "\n"))

	// Reconstruct conversational parts
	before := strings.TrimSpace(strings.Join(lines[:openIdx], "\n"))
	after := strings.TrimSpace(strings.Join(lines[closeIdx+1:], "\n"))

	var convParts []string
	if before != "" {
		convParts = append(convParts, before)
	}
	if after != "" {
		convParts = append(convParts, after)
	}

	return strings.Join(convParts, "\n\n"), docContent
}

// HandleReevaluateCommand checks if a message is the /reevaluate command.
func HandleReevaluateCommand(content string) bool {
	return strings.TrimSpace(content) == "/reevaluate"
}

// TimeNow returns the current time for response timestamps.
// Exposed for use by handlers.
func TimeNow() time.Time {
	return time.Now()
}
