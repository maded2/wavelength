package interview

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// LLMClient defines the subset of llm.Client needed by the interview service.
type LLMClient interface {
	Call(ctx context.Context, messages []llm.Message) (string, error)
	CallWithTools(ctx context.Context, messages []llm.Message, tools []*llm.Tool) (string, error)
	PersonaPrompt() string
}

// Service orchestrates the interview flow between a user, an LLM, and a topic store.
type Service struct {
	store    Store
	client   LLMClient
	dataDir  string
	mcpTools []*llm.Tool // Optional MCP tools from external servers
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
// dataDir is the root data directory (parent of topics/) used for file tool access.
// mcpTools are optional tools from external MCP servers.
func New(store Store, client LLMClient, dataDir string, mcpTools []*llm.Tool) *Service {
	return &Service{
		store:    store,
		client:   client,
		dataDir:  dataDir,
		mcpTools: mcpTools,
	}
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

	// Build tools for the LLM
	tools := s.buildTools(topicID)

	// Call the LLM with tool support
	messages := []llm.Message{
		{Role: "system", Content: s.client.PersonaPrompt()},
		{Role: "user", Content: prompt},
	}

	assistantResponse, err := s.client.CallWithTools(ctx, messages, tools)
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

	reevalPrompt := fmt.Sprintf(ReevaluationPrompt, topic.Name, description, document)

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

// buildTools creates the list of tools available to the LLM for this topic.
// Includes built-in file tools plus any configured MCP tools.
func (s *Service) buildTools(topicID string) []*llm.Tool {
	topicDir := filepath.Join(s.dataDir, "topics", topicID)
	tools := []*llm.Tool{
		llm.FileReadTool(topicDir),
		llm.WriteDocumentTool(topicDir, func(content string) {
			// Update the in-memory store when the LLM writes the document
			s.store.SetDocument(topicID, content)
		}),
	}

	// Add MCP tools if available
	if len(s.mcpTools) > 0 {
		tools = append(tools, s.mcpTools...)
	}

	return tools
}

// --- Private helpers (extracted from api/routes.go) ---

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
	buf.WriteString(fmt.Sprintf(TopicHeader, t.Name, description))

	// Include uploaded document attachments as reference material
	if len(t.Attachments) > 0 {
		buf.WriteString(UploadedDocsHeader)
		for _, att := range t.Attachments {
			if att.MarkdownContent == "" {
				continue
			}
			buf.WriteString(fmt.Sprintf(AttachmentHeader, att.Filename, att.Format, att.Size))
			content := att.MarkdownContent
			if len(content) > 8000 {
				content = content[:8000] + "\n" + TruncatedSuffix
			}
			buf.WriteString(content + "\n")
		}
		buf.WriteString("\n")
	}

	// Include current requirement document as context
	if t.Document != "" {
		buf.WriteString(CurrentDocHeader)
		doc := t.Document
		if len(doc) > 4000 {
			doc = doc[:4000] + "\n" + ContextTruncatedSuffix
		}
		buf.WriteString(doc + "\n")
	} else {
		buf.WriteString(NoDocYet)
	}

	// If conversation fits within limits, include everything
	if totalChars <= maxContextChars || len(t.Messages) <= maxRecentMessages {
		buf.WriteString(ConversationContextHeader)
		for _, msg := range t.Messages {
			fmt.Fprintf(&buf, MessageFormat, msg.Role, msg.Content)
		}

		// First interaction — instruct the LLM to critically evaluate existing information
		// before responding to the user's message.
		if len(t.Messages) == 1 {
			buf.WriteString(fmt.Sprintf(FirstInteractionPrompt, userMessage))
			return buf.String()
		}

		buf.WriteString(fmt.Sprintf(UserLatestMessagePrompt, userMessage))
		return buf.String()
	}

	// Summarize older messages, keep recent ones verbatim
	olderMessages := t.Messages[:len(t.Messages)-maxRecentMessages]
	recentMessages := t.Messages[len(t.Messages)-maxRecentMessages:]

	buf.WriteString(ConversationSummaryHeader)
	buf.WriteString(SummarizeMessages(olderMessages))

	buf.WriteString(RecentConversationHeader)
	for _, msg := range recentMessages {
		fmt.Fprintf(&buf, MessageFormat, msg.Role, msg.Content)
	}

	buf.WriteString(fmt.Sprintf(UserLatestMessagePrompt, userMessage))
	return buf.String()
}

// SummarizeMessages creates a compact summary of conversation messages.
func SummarizeMessages(messages []topic.Message) string {
	if len(messages) == 0 {
		return NoPriorConversation
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf(SummaryHeader, len(messages)))

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
		buf.WriteString(StakeholderKeyPointsHeader)
		for i, point := range userPoints {
			buf.WriteString(fmt.Sprintf(SummaryPointFormat, i+1, point))
		}
		buf.WriteString("\n")
	}

	if len(assistantInsights) > 0 {
		buf.WriteString(AnalystKeyInsightsHeader)
		for i, insight := range assistantInsights {
			buf.WriteString(fmt.Sprintf(SummaryPointFormat, i+1, insight))
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
