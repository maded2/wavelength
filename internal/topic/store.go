package topic

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Attachment represents an uploaded document attached to a topic.
type Attachment struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	Format    string    `json:"format"` // "markdown", "pdf", "word"
	Size      int64     `json:"size"`
	UploadedAt time.Time `json:"uploaded_at"`
	// MarkdownContent holds the converted markdown text (for LLM context)
	MarkdownContent string `json:"markdown_content,omitempty"`
}

// Topic represents a requirement-gathering initiative.
type Topic struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	Status        string       `json:"status"` // "not_started", "active", "completed"
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	MessageCount  int          `json:"message_count"`
	Messages      []Message    `json:"messages"`
	Document      string       `json:"document"`
	Attachments   []Attachment `json:"attachments,omitempty"`
}

// Message represents a single exchange in a conversation.
type Message struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// TopicStore is the interface for topic storage operations.
// Both in-memory (Store) and file-backed (FileStore) implementations satisfy it.
type TopicStore interface {
	Create(id, name, description string) *Topic
	Get(id string) *Topic
	List() []*Topic
	AddMessage(topicID, role, content string)
	ClearMessages(id string) bool
	SetStatus(id, status string) bool
	SetDocument(id, document string) bool
	Delete(id string) bool
	AddAttachment(topicID string, attachment Attachment) bool
	ListAttachments(topicID string) []Attachment
}

// blankDocument returns the initial markdown template for a new requirement document.
// The template is pre-populated with the topic name and description so the AI agent
// has a structured starting point to build upon during the interview.
func blankDocument(name, description string) string {
	return fmt.Sprintf("# Requirements: %s\n\n## Overview\n\n%s\n\n## Functional Requirements\n\n(To be elaborated during the interview)\n\n## Non-Functional Requirements\n\n(To be elaborated during the interview)\n\n## Stakeholders\n\n(To be identified during the interview)\n\n## Constraints\n\n(To be identified during the interview)\n\n## Open Questions\n\n(To be resolved during the interview)", name, description)
}

// Store manages topics in memory.
type Store struct {
	mu     sync.RWMutex
	topics map[string]*Topic
}

// NewStore creates a new in-memory topic store.
func NewStore() *Store {
	return &Store{
		topics: make(map[string]*Topic),
	}
}

// Create adds a new topic to the store.
func (s *Store) Create(id, name, description string) *Topic {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	topic := &Topic{
		ID:          id,
		Name:        name,
		Description: description,
		Status:      "not_started",
		CreatedAt:   now,
		UpdatedAt:   now,
		Messages:    []Message{},
		Document:    blankDocument(name, description),
	}
	s.topics[id] = topic
	return topic
}

// Get retrieves a topic by ID.
func (s *Store) Get(id string) *Topic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.topics[id]
}

// AddMessage appends a message to a topic's conversation history.
func (s *Store) AddMessage(topicID, role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, ok := s.topics[topicID]
	if !ok {
		return
	}

	topic.Messages = append(topic.Messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	topic.MessageCount = len(topic.Messages)
	topic.UpdatedAt = time.Now()

	if topic.Status == "not_started" {
		topic.Status = "active"
	}
}

// ClearMessages removes all messages from a topic, resetting the conversation history.
func (s *Store) ClearMessages(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, ok := s.topics[id]
	if !ok {
		return false
	}

	topic.Messages = []Message{}
	topic.MessageCount = 0
	topic.UpdatedAt = time.Now()
	return true
}

// SetStatus updates the status of a topic.
func (s *Store) SetStatus(id, status string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, ok := s.topics[id]
	if !ok {
		return false
	}

	topic.Status = status
	topic.UpdatedAt = time.Now()
	return true
}

// SetDocument updates the requirement document for a topic.
func (s *Store) SetDocument(id, document string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, ok := s.topics[id]
	if !ok {
		return false
	}

	topic.Document = document
	topic.UpdatedAt = time.Now()
	return true
}

// Delete removes a topic from the store by ID.
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.topics[id]; !ok {
		return false
	}

	delete(s.topics, id)
	return true
}

// AddAttachment adds an uploaded document attachment to a topic.
func (s *Store) AddAttachment(topicID string, attachment Attachment) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, ok := s.topics[topicID]
	if !ok {
		return false
	}

	topic.Attachments = append(topic.Attachments, attachment)
	topic.UpdatedAt = time.Now()
	return true
}

// ListAttachments returns all attachments for a topic.
func (s *Store) ListAttachments(topicID string) []Attachment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topic, ok := s.topics[topicID]
	if !ok {
		return nil
	}

	if topic.Attachments == nil {
		return []Attachment{}
	}
	return topic.Attachments
}

// List returns all topics, sorted by most recently updated first.
func (s *Store) List() []*Topic {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Topic, 0, len(s.topics))
	for _, t := range s.topics {
		result = append(result, t)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result
}
