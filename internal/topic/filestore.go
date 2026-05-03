package topic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// FileStore manages topics with file-based persistence.
type FileStore struct {
	mu      sync.RWMutex
	topics  map[string]*Topic
	dataDir string
}

// NewFileStore creates a new file-backed topic store.
func NewFileStore(dataDir string) *FileStore {
	return &FileStore{
		topics:  make(map[string]*Topic),
		dataDir: dataDir,
	}
}

// topicDir returns the path to the topics subdirectory.
func (s *FileStore) topicDir() string {
	return filepath.Join(s.dataDir, "topics")
}

// topicFile returns the file path for a given topic ID.
func (s *FileStore) topicFile(id string) string {
	return filepath.Join(s.topicDir(), id+".json")
}

// LoadAll reads all topics from disk into memory.
func (s *FileStore) LoadAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	td := s.topicDir()
	if _, err := os.Stat(td); os.IsNotExist(err) {
		return nil // No topics directory yet
	}

	entries, err := os.ReadDir(td)
	if err != nil {
		return fmt.Errorf("failed to read topics directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(td, entry.Name()))
		if err != nil {
			return fmt.Errorf("failed to read topic file %s: %w", entry.Name(), err)
		}

		var topic Topic
		if err := json.Unmarshal(data, &topic); err != nil {
			return fmt.Errorf("failed to parse topic file %s: %w", entry.Name(), err)
		}

		s.topics[topic.ID] = &topic
	}

	return nil
}

// SaveAll writes all topics to disk.
func (s *FileStore) SaveAll() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	td := s.topicDir()
	if err := os.MkdirAll(td, 0755); err != nil {
		return fmt.Errorf("failed to create topics directory: %w", err)
	}

	for id, topic := range s.topics {
		data, err := json.MarshalIndent(topic, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal topic %s: %w", id, err)
		}

		if err := os.WriteFile(s.topicFile(id), data, 0644); err != nil {
			return fmt.Errorf("failed to write topic file %s: %w", id, err)
		}
	}

	return nil
}

// Create adds a new topic to the store.
func (s *FileStore) Create(id, name, description string) *Topic {
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
func (s *FileStore) Get(id string) *Topic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	topic, ok := s.topics[id]
	if !ok {
		return nil
	}
	// Return a copy to avoid race conditions
	topicCopy := *topic
	return &topicCopy
}

// AddMessage appends a message to a topic's conversation history.
func (s *FileStore) AddMessage(topicID, role, content string) {
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

// List returns all topics, sorted by most recently updated first.
func (s *FileStore) List() []*Topic {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Topic, 0, len(s.topics))
	for _, t := range s.topics {
		topicCopy := *t
		result = append(result, &topicCopy)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result
}
