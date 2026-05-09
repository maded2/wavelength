package topic

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// FileStore manages topics with file-based persistence.
// Each topic is stored as a directory:
//
//	data/topics/<topic-id>/
//	    meta.json      — topic metadata
//	    messages.jsonl — conversation messages (one JSON per line)
//	    document.md    — living requirement document (plain text)
//
// File locking ensures concurrent access safety:
// - A global lock file protects LoadAll and SaveAll operations
// - Per-topic locks protect individual topic reads/writes
// - Atomic writes (write-to-temp + rename) prevent corruption on crash
type FileStore struct {
	mu      sync.RWMutex
	topics  map[string]*Topic
	dataDir string
	lock    *flock.Flock // Global lock for data directory operations
}

// NewFileStore creates a new file-backed topic store.
func NewFileStore(dataDir string) *FileStore {
	return &FileStore{
		topics:  make(map[string]*Topic),
		dataDir: dataDir,
		lock:    flock.New(filepath.Join(dataDir, ".data.lock")),
	}
}

// lockData acquires the global data directory lock.
func (s *FileStore) lockData() error {
	const timeout = 10 * time.Second
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.lock.Lock()
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout acquiring data directory lock after %v", timeout)
	}
}

// unlockData releases the global data directory lock.
func (s *FileStore) unlockData() {
	_ = s.lock.Unlock()
}

// topicLockFile returns the path to a per-topic lock file.
func (s *FileStore) topicLockFile(id string) string {
	return filepath.Join(s.topicPath(id), ".topic.lock")
}

// lockTopic acquires a per-topic lock for safe concurrent access.
func (s *FileStore) lockTopic(id string) (*flock.Flock, error) {
	tPath := s.topicPath(id)
	if err := os.MkdirAll(tPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create topic directory for lock: %w", err)
	}

	topicLock := flock.New(s.topicLockFile(id))
	const timeout = 5 * time.Second
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = topicLock.Lock()
	}()

	select {
	case <-done:
		return topicLock, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout acquiring topic lock for %q after %v", id, timeout)
	}
}

// atomicWriteFile writes data to a temp file then atomically renames it to the target path.
// This prevents corruption if the process crashes mid-write.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for atomic write: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()

	_, err = tmpFile.Write(data)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to rename temp file to target: %w", err)
	}

	return nil
}

// topicsRoot returns the path to the top-level topics directory.
func (s *FileStore) topicsRoot() string {
	return filepath.Join(s.dataDir, "topics")
}

// topicPath returns the directory path for a given topic ID.
func (s *FileStore) topicPath(id string) string {
	return filepath.Join(s.topicsRoot(), id)
}

// metaFile returns the path to the topic's metadata JSON file.
func (s *FileStore) metaFile(id string) string {
	return filepath.Join(s.topicPath(id), "meta.json")
}

// messagesFile returns the path to the topic's messages JSONL file.
func (s *FileStore) messagesFile(id string) string {
	return filepath.Join(s.topicPath(id), "messages.jsonl")
}

// documentFile returns the path to the topic's requirement document file.
func (s *FileStore) documentFile(id string) string {
	return filepath.Join(s.topicPath(id), "document.md")
}

// topicMeta holds the serializable metadata for a topic (no messages or document).
type topicMeta struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	MessageCount int     `json:"message_count"`
}

// topicMessageLine is the JSONL format for a single message.
type topicMessageLine struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// LoadAll reads all topics from disk into memory.
// It handles both the new directory-per-topic format and the legacy single-file format.
// Acquires a global lock to prevent concurrent reads during startup.
func (s *FileStore) LoadAll() error {
	if err := s.lockData(); err != nil {
		return fmt.Errorf("failed to acquire data lock: %w", err)
	}
	defer s.unlockData()

	s.mu.Lock()
	defer s.mu.Unlock()

	root := s.topicsRoot()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil // No topics directory yet
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("failed to read topics directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		entryPath := filepath.Join(root, name)

		if entry.IsDir() {
			// New directory-per-topic format
			if err := s.loadTopicDir(name, entryPath); err != nil {
				return fmt.Errorf("failed to load topic %s: %w", name, err)
			}
		} else if strings.HasSuffix(name, ".json") {
			// Legacy single-file format — migrate on the fly
			id := strings.TrimSuffix(name, ".json")
			if err := s.loadTopicLegacy(id, entryPath); err != nil {
				return fmt.Errorf("failed to load legacy topic %s: %w", id, err)
			}
		}
	}

	return nil
}

// loadTopicDir loads a topic from its directory.
func (s *FileStore) loadTopicDir(id, dirPath string) error {
	var meta topicMeta

	// Load metadata
	metaData, err := os.ReadFile(filepath.Join(dirPath, "meta.json"))
	if err != nil {
		return fmt.Errorf("failed to read meta.json: %w", err)
	}
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return fmt.Errorf("failed to parse meta.json: %w", err)
	}

	// Load messages
	messages, err := s.readMessagesFile(filepath.Join(dirPath, "messages.jsonl"))
	if err != nil {
		return fmt.Errorf("failed to read messages: %w", err)
	}

	// Load document (optional — file may not exist yet)
	var document string
	docData, err := os.ReadFile(filepath.Join(dirPath, "document.md"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read document.md: %w", err)
	}
	if err == nil {
		document = string(docData)
	}

	topic := &Topic{
		ID:          meta.ID,
		Name:        meta.Name,
		Description: meta.Description,
		Status:      meta.Status,
		CreatedAt:   meta.CreatedAt,
		UpdatedAt:   meta.UpdatedAt,
		MessageCount: meta.MessageCount,
		Messages:    messages,
		Document:    document,
	}

	s.topics[id] = topic
	return nil
}

// loadTopicLegacy loads a topic from the old single-file JSON format and migrates it.
func (s *FileStore) loadTopicLegacy(id, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read legacy topic file: %w", err)
	}

	var topic Topic
	if err := json.Unmarshal(data, &topic); err != nil {
		return fmt.Errorf("failed to parse legacy topic file: %w", err)
	}

	s.topics[id] = &topic

	// Migrate to directory format in the background (non-blocking, best-effort)
	go func() {
		if err := s.migrateTopicToDir(&topic); err != nil {
			// Log but don't fail — topic is still loaded in memory
			// The migration will be retried on next SaveAll
		}
	}()

	return nil
}

// migrateTopicToDir converts a legacy single-file topic to the directory format.
func (s *FileStore) migrateTopicToDir(topic *Topic) error {
	tPath := s.topicPath(topic.ID)
	if err := os.MkdirAll(tPath, 0755); err != nil {
		return fmt.Errorf("failed to create topic directory: %w", err)
	}

	// Write meta.json
	meta := topicMeta{
		ID:          topic.ID,
		Name:        topic.Name,
		Description: topic.Description,
		Status:      topic.Status,
		CreatedAt:   topic.CreatedAt,
		UpdatedAt:   topic.UpdatedAt,
		MessageCount: topic.MessageCount,
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal meta: %w", err)
	}
	if err := os.WriteFile(s.metaFile(topic.ID), metaData, 0644); err != nil {
		return fmt.Errorf("failed to write meta.json: %w", err)
	}

	// Write messages.jsonl
	if err := s.writeMessagesFile(s.messagesFile(topic.ID), topic.Messages); err != nil {
		return fmt.Errorf("failed to write messages.jsonl: %w", err)
	}

	// Write document.md (only if non-empty)
	if topic.Document != "" {
		if err := os.WriteFile(s.documentFile(topic.ID), []byte(topic.Document), 0644); err != nil {
			return fmt.Errorf("failed to write document.md: %w", err)
		}
	}

	// Remove the legacy file
	legacyFile := filepath.Join(s.topicsRoot(), topic.ID+".json")
	_ = os.Remove(legacyFile)

	return nil
}

// readMessagesFile reads a JSONL file and returns the messages.
func (s *FileStore) readMessagesFile(path string) ([]Message, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Message{}, nil // No messages file yet
		}
		return nil, err
	}
	defer f.Close()

	var messages []Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for long messages
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg topicMessageLine
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return nil, fmt.Errorf("failed to parse message line: %w: %s", err, line[:min(len(line), 200)])
		}
		messages = append(messages, Message{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return messages, nil
}

// writeMessagesFile writes all messages to a JSONL file.
func (s *FileStore) writeMessagesFile(path string, messages []Message) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, msg := range messages {
		line, err := json.Marshal(topicMessageLine{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		})
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		if _, err := w.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("failed to write message line: %w", err)
		}
	}
	return w.Flush()
}

// appendMessageToFile appends a single message to the JSONL file.
func (s *FileStore) appendMessageToFile(path string, msg Message) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	line, err := json.Marshal(topicMessageLine{
		Role:      msg.Role,
		Content:   msg.Content,
		Timestamp: msg.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = f.Write(append(line, '\n'))
	return err
}

// SaveAll writes all topics to disk in the directory-per-topic format.
// Acquires a global lock to prevent concurrent saves.
func (s *FileStore) SaveAll() error {
	if err := s.lockData(); err != nil {
		return fmt.Errorf("failed to acquire data lock: %w", err)
	}
	defer s.unlockData()

	s.mu.RLock()
	defer s.mu.RUnlock()

	root := s.topicsRoot()
	if err := os.MkdirAll(root, 0755); err != nil {
		return fmt.Errorf("failed to create topics directory: %w", err)
	}

	for id, topic := range s.topics {
		if err := s.saveTopicDir(id, topic); err != nil {
			return fmt.Errorf("failed to save topic %s: %w", id, err)
		}
	}

	return nil
}

// saveTopicDir writes a topic to its directory using atomic writes.
func (s *FileStore) saveTopicDir(id string, topic *Topic) error {
	tPath := s.topicPath(id)
	if err := os.MkdirAll(tPath, 0755); err != nil {
		return fmt.Errorf("failed to create topic directory: %w", err)
	}

	// Write meta.json atomically
	meta := topicMeta{
		ID:          topic.ID,
		Name:        topic.Name,
		Description: topic.Description,
		Status:      topic.Status,
		CreatedAt:   topic.CreatedAt,
		UpdatedAt:   topic.UpdatedAt,
		MessageCount: topic.MessageCount,
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal meta: %w", err)
	}
	if err := atomicWriteFile(s.metaFile(id), metaData, 0644); err != nil {
		return fmt.Errorf("failed to write meta.json: %w", err)
	}

	// Write messages.jsonl atomically
	msgData, err := s.messagesToJSONL(topic.Messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}
	if err := atomicWriteFile(s.messagesFile(id), msgData, 0644); err != nil {
		return fmt.Errorf("failed to write messages.jsonl: %w", err)
	}

	// Write document.md atomically (only if non-empty)
	if topic.Document != "" {
		if err := atomicWriteFile(s.documentFile(id), []byte(topic.Document), 0644); err != nil {
			return fmt.Errorf("failed to write document.md: %w", err)
		}
	}

	return nil
}

// messagesToJSONL serializes messages to JSONL format.
func (s *FileStore) messagesToJSONL(messages []Message) ([]byte, error) {
	var buf strings.Builder
	for _, msg := range messages {
		line, err := json.Marshal(topicMessageLine{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal message: %w", err)
		}
		buf.Write(append(line, '\n'))
	}
	return []byte(buf.String()), nil
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
		Document:    blankDocument(name, description),
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
	// Return a deep copy to avoid race conditions
	topicCopy := deepCopyTopic(topic)
	return &topicCopy
}

// deepCopyTopic creates a full deep copy of a topic.
func deepCopyTopic(t *Topic) Topic {
	copied := *t
	if t.Messages != nil {
		copied.Messages = make([]Message, len(t.Messages))
		copy(copied.Messages, t.Messages)
	}
	return copied
}

// AddMessage appends a message to a topic's conversation history.
func (s *FileStore) AddMessage(topicID, role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, ok := s.topics[topicID]
	if !ok {
		return
	}

	msg := Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	topic.Messages = append(topic.Messages, msg)
	topic.MessageCount = len(topic.Messages)
	topic.UpdatedAt = time.Now()

	if topic.Status == "not_started" {
		topic.Status = "active"
	}

	// Persist the new message to disk immediately (append to JSONL)
	_ = s.persistTopicUpdate(topicID, topic)
}

// ClearMessages removes all messages from a topic, resetting the conversation history.
func (s *FileStore) ClearMessages(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, ok := s.topics[id]
	if !ok {
		return false
	}

	topic.Messages = []Message{}
	topic.MessageCount = 0
	topic.UpdatedAt = time.Now()

	// Persist: rewrite messages.jsonl as empty
	_ = s.writeMessagesFile(s.messagesFile(id), []Message{})
	_ = s.persistTopicUpdate(id, topic)
	return true
}

// SetStatus updates the status of a topic and persists it to disk.
func (s *FileStore) SetStatus(id, status string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, ok := s.topics[id]
	if !ok {
		return false
	}

	topic.Status = status
	topic.UpdatedAt = time.Now()
	_ = s.persistTopicUpdate(id, topic)
	return true
}

// SetDocument updates the requirement document for a topic and persists it to disk.
func (s *FileStore) SetDocument(id, document string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, ok := s.topics[id]
	if !ok {
		return false
	}

	topic.Document = document
	topic.UpdatedAt = time.Now()
	_ = s.persistTopicUpdate(id, topic)
	return true
}

// persistTopicUpdate writes the updated topic to disk. Must be called while holding the write lock.
func (s *FileStore) persistTopicUpdate(id string, topic *Topic) error {
	tPath := s.topicPath(id)
	if err := os.MkdirAll(tPath, 0755); err != nil {
		return fmt.Errorf("failed to create topic directory: %w", err)
	}

	// Write meta.json
	meta := topicMeta{
		ID:          topic.ID,
		Name:        topic.Name,
		Description: topic.Description,
		Status:      topic.Status,
		CreatedAt:   topic.CreatedAt,
		UpdatedAt:   topic.UpdatedAt,
		MessageCount: topic.MessageCount,
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal meta: %w", err)
	}
	if err := os.WriteFile(s.metaFile(id), metaData, 0644); err != nil {
		return fmt.Errorf("failed to write meta.json: %w", err)
	}

	// Write document.md (only if non-empty)
	if topic.Document != "" {
		if err := os.WriteFile(s.documentFile(id), []byte(topic.Document), 0644); err != nil {
			return fmt.Errorf("failed to write document.md: %w", err)
		}
	}

	return nil
}

// Delete removes a topic from the store and deletes its directory from disk.
func (s *FileStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.topics[id]; !ok {
		return false
	}

	delete(s.topics, id)
	_ = os.RemoveAll(s.topicPath(id))
	// Also remove any legacy file that might still exist
	_ = os.Remove(filepath.Join(s.topicsRoot(), id+".json"))
	return true
}

// List returns all topics, sorted by most recently updated first.
func (s *FileStore) List() []*Topic {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Topic, 0, len(s.topics))
	for _, t := range s.topics {
		topicCopy := deepCopyTopic(t)
		result = append(result, &topicCopy)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
