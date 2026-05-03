package topic

import (
	"sort"
	"sync"
	"time"
)

// Topic represents a requirement-gathering initiative.
type Topic struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Status        string     `json:"status"` // "not_started", "active", "completed"
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	MessageCount  int        `json:"message_count"`
	Messages      []Message  `json:"messages"`
	Document      string     `json:"document"`
}

// Message represents a single exchange in a conversation.
type Message struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
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
