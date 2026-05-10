package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"wavelength/internal/convert"
	"wavelength/internal/export"
	"wavelength/internal/llm"
	topicpkg "wavelength/internal/topic"
)

// SetupRoutes registers all API routes on the given Fiber app.
func SetupRoutes(app *fiber.App, store topicpkg.TopicStore, client *llm.Client) {
	// Static landing page
	app.Get("/", LandingPage)

	// Topic chat page (serves the UI, not JSON)
	app.Get("/topics/:id", TopicPage)

	// Health check
	app.Get("/health", HealthHandler(client))

	// Topic CRUD
	app.Get("/api/topics", func(c *fiber.Ctx) error {
		topics := store.List()
		result := make([]map[string]interface{}, 0, len(topics))
		for _, t := range topics {
			result = append(result, map[string]interface{}{
				"id":            t.ID,
				"name":          t.Name,
				"description":   t.Description,
				"status":        t.Status,
				"created_at":    t.CreatedAt,
				"updated_at":    t.UpdatedAt,
				"message_count": t.MessageCount,
			})
		}
		return c.JSON(result)
	})

	app.Post("/api/topics", func(c *fiber.Ctx) error {
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Document    string `json:"document"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"message": "invalid request body",
			})
		}

		if req.Name == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"message": "topic name is required",
			})
		}

		if req.Description == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"message": "high-level description is required — this is what the AI agent will use to begin the interview",
			})
		}

		// Check for duplicate names
		for _, t := range store.List() {
			if t.Name == req.Name {
				return c.Status(http.StatusConflict).JSON(fiber.Map{
					"message": fmt.Sprintf("a topic with the name %q already exists. Please choose a different name.", req.Name),
				})
			}
		}

		id := fmt.Sprintf("topic-%d", time.Now().UnixNano())
		topic := store.Create(id, req.Name, req.Description)

		// Set pre-existing document if provided
		if req.Document != "" {
			store.SetDocument(id, req.Document)
			topic = store.Get(id)
		}

		return c.Status(http.StatusCreated).JSON(topic)
	})

	app.Get("/api/topics/:id", func(c *fiber.Ctx) error {
		topic := store.Get(c.Params("id"))
		if topic == nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"message": "topic not found",
			})
		}
		return c.JSON(topic)
	})

	app.Delete("/api/topics/:id", func(c *fiber.Ctx) error {
		topicID := c.Params("id")
		topic := store.Get(topicID)
		if topic == nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"message": "topic not found",
			})
		}

		store.Delete(topicID)
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"message": "topic deleted",
		})
	})

	// Update topic status (e.g., mark as completed or reopen)
	app.Patch("/api/topics/:id", func(c *fiber.Ctx) error {
		topicID := c.Params("id")
		topic := store.Get(topicID)
		if topic == nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"message": "topic not found",
			})
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"message": "invalid request body",
			})
		}

		if req.Status == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"message": "status is required",
			})
		}

		store.SetStatus(topicID, req.Status)

		updated := store.Get(topicID)
		return c.JSON(updated)
	})

	// Update topic requirement document
	app.Patch("/api/topics/:id/document", func(c *fiber.Ctx) error {
		topicID := c.Params("id")
		topic := store.Get(topicID)
		if topic == nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"message": "topic not found",
			})
		}

		var req struct {
			Document string `json:"document"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"message": "invalid request body",
			})
		}

		store.SetDocument(topicID, req.Document)

		updated := store.Get(topicID)
		return c.JSON(updated)
	})

	// Download topic requirement document in various formats
	app.Get("/api/topics/:id/document/download", func(c *fiber.Ctx) error {
		topicID := c.Params("id")
		topic := store.Get(topicID)
		if topic == nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"message": "topic not found",
			})
		}

		// Default to markdown, accept ?format=pdf or ?format=word
		format := export.Format(strings.ToLower(c.Query("format", "markdown")))

		exp := export.New(topic.Document)
		data, mimeType, ext, err := exp.Export(format)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		filename := strings.ReplaceAll(topic.Name, " ", "_")
		// Sanitize filename
		filename = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
				return r
			}
			return '_'
		}, filename)

		// Trigger browser file download with proper headers
		c.Set("Content-Type", mimeType)
		c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s%s"`, filename, ext))
		c.Set("Content-Length", fmt.Sprintf("%d", len(data)))
		return c.Send(data)
	})

// Upload document to topic (multipart form)
		app.Post("/api/topics/:id/upload", func(c *fiber.Ctx) error {
			topicID := c.Params("id")
			topic := store.Get(topicID)
			if topic == nil {
				return c.Status(http.StatusNotFound).JSON(fiber.Map{
					"message": "topic not found",
				})
			}

			// Block uploads on completed topics
			if topic.Status == "completed" {
				return c.Status(http.StatusConflict).JSON(fiber.Map{
					"message": "this topic is marked as complete. Please reopen it before uploading documents.",
				})
			}

			// Parse multipart form (max 10MB)
			form, err := c.MultipartForm()
			if err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"message": "failed to parse upload: " + err.Error(),
				})
			}

			files := form.File["file"]
			if len(files) == 0 {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"message": "no file provided. Use form field name 'file'.",
				})
			}

			file := files[0]

			// Check file size (max 10MB)
			if file.Size > convert.MaxUploadSize {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"message": fmt.Sprintf("file too large: %d bytes (max %d bytes)", file.Size, convert.MaxUploadSize),
				})
			}

			// Open uploaded file
			src, err := file.Open()
			if err != nil {
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
					"message": "failed to open uploaded file: " + err.Error(),
				})
			}
			defer src.Close()

			// Convert to markdown
			conv := convert.New()
			markdown, err := conv.Convert(src, file.Filename)
			if err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"message": "failed to convert document: " + err.Error(),
				})
			}

			// Determine format
			format, _ := convert.DetectFormat(file.Filename)

			// Create attachment record
			attachment := topicpkg.Attachment{
				ID:              fmt.Sprintf("att-%s", uuid.New().String()[:8]),
				Filename:        file.Filename,
				Format:          string(format),
				Size:            file.Size,
				UploadedAt:      time.Now(),
				MarkdownContent: markdown,
			}

			// Store attachment
			if !store.AddAttachment(topicID, attachment) {
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
					"message": "failed to save attachment",
				})
			}

			log.Printf("[UPLOAD] File %q uploaded to topic %q (%d bytes, %d chars markdown)", file.Filename, topicID, file.Size, len(markdown))

			return c.Status(http.StatusCreated).JSON(fiber.Map{
				"success":    true,
				"attachment": fiber.Map{
					"id":         attachment.ID,
					"filename":   attachment.Filename,
					"format":     attachment.Format,
					"size":       attachment.Size,
					"chars":      len(markdown),
					"uploaded_at": attachment.UploadedAt,
				},
				"message": fmt.Sprintf("Document %q uploaded and converted to markdown (%d characters). The AI agent can now reference this document.", filepath.Base(file.Filename), len(markdown)),
			})
		})

	// List attachments for a topic
		app.Get("/api/topics/:id/attachments", func(c *fiber.Ctx) error {
			topicID := c.Params("id")
			topic := store.Get(topicID)
			if topic == nil {
				return c.Status(http.StatusNotFound).JSON(fiber.Map{
					"message": "topic not found",
				})
			}

			attachments := store.ListAttachments(topicID)
			// Return only metadata, not markdown content (too large for list)
			result := make([]map[string]interface{}, 0, len(attachments))
			for _, att := range attachments {
				result = append(result, map[string]interface{}{
					"id":          att.ID,
					"filename":    att.Filename,
					"format":      att.Format,
					"size":        att.Size,
					"uploaded_at": att.UploadedAt,
				})
			}
			return c.JSON(result)
		})

	// Submit message to topic conversation (non-streaming, legacy)
		app.Post("/api/topics/:id/messages", func(c *fiber.Ctx) error {
		topicID := c.Params("id")

		var req struct {
			Content string `json:"content"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"message": "invalid request body",
			})
		}

		if req.Content == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"message": "message content is required",
			})
		}

		// Check topic exists and is not completed
		topic := store.Get(topicID)
		if topic == nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"message": "topic not found",
			})
		}

		// Block messages on completed topics — they must be reopened first
		if topic.Status == "completed" {
			return c.Status(http.StatusConflict).JSON(fiber.Map{
				"message": "this topic is marked as complete. Please reopen it before adding new messages.",
			})
		}

		// Handle /reevaluate command: clear history and re-assess the document
		if strings.TrimSpace(req.Content) == "/reevaluate" {
			return handleReevaluate(c, store, client, topicID, topic)
		}

		// Always save the user's message first
		store.AddMessage(topicID, "user", req.Content)

		// Re-fetch topic so it includes the newly added user message
		// (FileStore.Get returns a copy, so the old reference is stale)
		topic = store.Get(topicID)

		// Try to get AI agent response
		assistantResponse, err := generateResponse(client, topic, req.Content)
		if err != nil {
			// LLM failed — return user-friendly error but user message is preserved
			return c.Status(http.StatusOK).JSON(fiber.Map{
				"success": false,
				"error":   "The AI agent is temporarily unavailable. Your message has been saved. Please try again later.",
				"message": fiber.Map{
					"role":      "user",
					"content":   req.Content,
					"timestamp": time.Now(),
				},
			})
		}

		// Extract any embedded requirements document from the LLM response
		conversationalPart, extractedDoc := extractDocument(assistantResponse)

		// If a document was extracted, update the topic's requirement document
		documentUpdated := false
		if extractedDoc != "" {
			if store.SetDocument(topicID, extractedDoc) {
				documentUpdated = true
				log.Printf("[DOC] Updated requirement document for topic %q (%d bytes)", topicID, len(extractedDoc))
			}
		}

		// Save assistant response (the conversational portion)
		store.AddMessage(topicID, "assistant", conversationalPart)

		return c.Status(http.StatusOK).JSON(fiber.Map{
			"success":         true,
			"message":         fiber.Map{
				"role":      "assistant",
				"content":   conversationalPart,
				"timestamp": time.Now(),
			},
			"document_updated": documentUpdated,
		})
	})

	// Streaming message endpoint — returns SSE stream of tokens
	app.Post("/api/topics/:id/messages/stream", func(c *fiber.Ctx) error {
		topicID := c.Params("id")

		var req struct {
			Content string `json:"content"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"message": "invalid request body",
			})
		}

		if req.Content == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"message": "message content is required",
			})
		}

		// Check topic exists and is not completed
		topic := store.Get(topicID)
		if topic == nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"message": "topic not found",
			})
		}

		if topic.Status == "completed" {
			return c.Status(http.StatusConflict).JSON(fiber.Map{
				"message": "this topic is marked as complete. Please reopen it before adding new messages.",
			})
		}

		// Handle /reevaluate command — use non-streaming path
		if strings.TrimSpace(req.Content) == "/reevaluate" {
			return handleReevaluate(c, store, client, topicID, topic)
		}

		// Save user's message first
		store.AddMessage(topicID, "user", req.Content)

		// Re-fetch topic with user message included
		topic = store.Get(topicID)

		// Set SSE headers
		c.Context().SetContentType("text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no")

		// Build conversation messages for streaming (includes attachments in context)
		prompt := buildConversationContext(topic, req.Content)

		// Build messages array for streaming
		messages := make([]llm.Message, 0, len(topic.Messages))
		for _, msg := range topic.Messages {
			messages = append(messages, llm.Message{Role: msg.Role, Content: msg.Content})
		}
		messages = append(messages, llm.Message{Role: "user", Content: prompt})

		// Create a pipe to capture the full response for document extraction
		pr, pw := io.Pipe()

		// Start streaming to client in a goroutine
		go func() {
			defer pw.Close()

			// Send start event
			startEvent := map[string]interface{}{
				"type": "start",
			}
			json.NewEncoder(pw).Encode(startEvent)

			// Stream LLM response
			streamErr := client.StreamResponse(c.Context(), pw, client.PersonaPrompt(), messages)
			if streamErr != nil {
				errEvent := map[string]interface{}{
					"type":    "error",
					"message": "The AI agent is temporarily unavailable. Your message has been saved.",
				}
				json.NewEncoder(pw).Encode(errEvent)
			}

			// Send done event
			doneEvent := map[string]interface{}{
				"type": "done",
			}
			json.NewEncoder(pw).Encode(doneEvent)
		}()

		// Read from pipe to capture full response for document extraction
		go func() {
			var fullResponse strings.Builder
			buf := make([]byte, 4096)
			for {
				n, readErr := pr.Read(buf)
				if n > 0 {
					fullResponse.Write(buf[:n])
				}
				if readErr != nil {
					// Stream ended
					break
				}
			}

			// Parse tokens from captured response to reconstruct full assistant message
			responseText := fullResponse.String()
			var tokens []string
			scanner := bufio.NewScanner(strings.NewReader(responseText))
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}
				var event map[string]interface{}
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					continue
				}
				if eventType, ok := event["type"].(string); ok && eventType == "token" {
					if content, ok := event["content"].(string); ok {
						tokens = append(tokens, content)
					}
				}
			}

			assistantResponse := strings.Join(tokens, "")
			if assistantResponse == "" {
				return
			}

			// Extract any embedded requirements document
			conversationalPart, extractedDoc := extractDocument(assistantResponse)

			// Update document if extracted
			if extractedDoc != "" {
				if store.SetDocument(topicID, extractedDoc) {
					log.Printf("[DOC] Updated requirement document for topic %q (%d bytes)", topicID, len(extractedDoc))
				}
			}

			// Save assistant response
			store.AddMessage(topicID, "assistant", conversationalPart)
		}()

		// Stream the pipe to the HTTP response
		return c.SendStream(pr)
	})
}

// handleReevaluate clears the conversation history and asks the LLM to re-assess
// the current requirement document from scratch.
func handleReevaluate(c *fiber.Ctx, store topicpkg.TopicStore, client *llm.Client, topicID string, topic *topicpkg.Topic) error {
	log.Printf("[REEVAL] Re-evaluating topic %q — clearing %d messages", topicID, len(topic.Messages))

	// Clear all conversation history
	store.ClearMessages(topicID)

	// Build a re-evaluation prompt using only the topic name, description, and current document
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
3. Provide an updated version of the document wrapped in --- delimiters if changes are needed

Remember to maintain the document in markdown format and wrap any updated document in --- delimiters.`,
		topic.Name, description, document,
	)

	// Send re-evaluation prompt to the LLM
	assistantResponse, err := generateReevaluateResponse(client, topic, reevalPrompt)
	if err != nil {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"success": false,
			"error":   "The AI agent is temporarily unavailable. Please try again later.",
			"message": fiber.Map{
				"role":      "assistant",
				"content":   "Re-evaluation failed. The AI agent is temporarily unavailable.",
				"timestamp": time.Now(),
			},
		})
	}

	// Extract any embedded requirements document from the LLM response
	conversationalPart, extractedDoc := extractDocument(assistantResponse)

	// If a document was extracted, update the topic's requirement document
	documentUpdated := false
	if extractedDoc != "" {
		if store.SetDocument(topicID, extractedDoc) {
			documentUpdated = true
			log.Printf("[DOC] Updated requirement document for topic %q after re-evaluation (%d bytes)", topicID, len(extractedDoc))
		}
	}

	// Save assistant response
	store.AddMessage(topicID, "assistant", conversationalPart)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success":          true,
		"reevaluate":       true,
		"message":          fiber.Map{
			"role":      "assistant",
			"content":   conversationalPart,
			"timestamp": time.Now(),
		},
		"document_updated": documentUpdated,
	})
}

// generateReevaluateResponse sends a re-evaluation prompt to the LLM without conversation history.
func generateReevaluateResponse(client *llm.Client, t *topicpkg.Topic, prompt string) (string, error) {
	log.Printf("[REEVAL] Sending re-evaluation request for topic %q", t.ID)

	timeout := time.Duration(client.Timeout()) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	payload := map[string]interface{}{
		"model": client.Model(),
		"messages": []map[string]string{
			{"role": "system", "content": client.PersonaPrompt()},
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to prepare LLM request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", client.APIURL(), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("cannot connect to LLM service: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.APIKey())
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot connect to LLM service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("LLM service error: status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %v", err)
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("LLM returned no response")
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid LLM response format")
	}

	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid LLM response format")
	}

	content, ok := message["content"].(string)
	if !ok {
		return "", fmt.Errorf("LLM returned empty content")
	}

	log.Printf("[REEVAL] Re-evaluation response received: %d chars", len(content))
	return content, nil
}

// maxContextChars is the approximate character limit for conversation context.
// This is a conservative estimate to stay well within typical LLM context windows.
const maxContextChars = 60000

// maxRecentMessages is how many recent messages to keep verbatim when summarizing.
const maxRecentMessages = 20

// buildConversationContext constructs the conversation context string, applying
// summarization when the conversation exceeds maxContextChars.
//
// Strategy:
// 1. Keep the most recent N messages verbatim (maxRecentMessages)
// 2. Summarize all older messages into a compact summary
// 3. Always include topic name, description, current document, and uploaded attachments
// 4. Insert summary as a synthetic "narrator" message to preserve context
func buildConversationContext(t *topicpkg.Topic, userMessage string) string {
	description := t.Description
	if description == "" {
		description = "(no description provided)"
	}

	// Calculate total conversation size
	totalChars := 0
	for _, msg := range t.Messages {
		totalChars += len(msg.Content) + 20 // +20 for "role: " prefix and newline
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
			buf.WriteString(fmt.Sprintf("\n--- Document: %s (%s, %d bytes) ---\n", att.Filename, att.Format, att.Size))
			content := att.MarkdownContent
			// Truncate very large documents to fit context window
			maxDocChars := 8000
			if len(content) > maxDocChars {
				content = content[:maxDocChars] + "\n...(truncated)"
			}
			buf.WriteString(content)
			buf.WriteString("\n")
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
		buf.WriteString(doc)
		buf.WriteString("\n")
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

	// Build summary of older conversation
	buf.WriteString("\nConversation summary (earlier exchanges):\n")
	summary := summarizeMessages(olderMessages)
	buf.WriteString(summary)

	// Append recent messages verbatim
	buf.WriteString("\n\nRecent conversation:\n")
	for _, msg := range recentMessages {
		fmt.Fprintf(&buf, "%s: %s\n", msg.Role, msg.Content)
	}

	buf.WriteString(fmt.Sprintf("\nUser's latest message: %s\n\nPlease respond as a business analyst conducting requirements gathering.", userMessage))
	return buf.String()
}

// summarizeMessages creates a compact summary of conversation messages.
// It extracts key decisions, requirements mentioned, and important clarifications.
func summarizeMessages(messages []topicpkg.Message) string {
	if len(messages) == 0 {
		return "(no prior conversation)"
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Summary of %d earlier exchanges:\n\n", len(messages)))

	// Track key themes and decisions
	var userPoints []string
	var assistantInsights []string

	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if len(content) == 0 {
			continue
		}

		// Truncate very long messages for summary
		summaryLen := 200
		if len(content) > summaryLen {
			content = content[:summaryLen] + "..."
		}

		if msg.Role == "user" {
			userPoints = append(userPoints, content)
		} else {
			assistantInsights = append(assistantInsights, content)
		}
	}

	// Write user contributions
	if len(userPoints) > 0 {
		buf.WriteString("Key points from stakeholder:\n")
		for i, point := range userPoints {
			buf.WriteString(fmt.Sprintf("  %d. %s\n", i+1, point))
		}
		buf.WriteString("\n")
	}

	// Write assistant insights
	if len(assistantInsights) > 0 {
		buf.WriteString("Key insights and questions from analyst:\n")
		for i, insight := range assistantInsights {
			buf.WriteString(fmt.Sprintf("  %d. %s\n", i+1, insight))
		}
	}

	return buf.String()
}

// generateResponse calls the LLM to generate a response based on the conversation history.
// It applies context management (summarization) when conversations grow large.
func generateResponse(client *llm.Client, t *topicpkg.Topic, userMessage string) (string, error) {
	log.Printf("[LLM] === Request for topic %q ===", t.ID)

	timeout := time.Duration(client.Timeout()) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Build conversation context with automatic summarization for long conversations
	prompt := buildConversationContext(t, userMessage)

	log.Printf("[LLM] Prompt (truncated): %.500q", prompt)

	payload := map[string]interface{}{
		"model": client.Model(),
		"messages": []map[string]string{
			{"role": "system", "content": client.PersonaPrompt()},
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[LLM] ERROR marshalling payload: %v", err)
		return "", fmt.Errorf("failed to prepare LLM request: %v", err)
	}

	apiURL := client.APIURL()
	log.Printf("[LLM] Sending POST to %s (body size: %d bytes)", apiURL, len(body))
	log.Printf("[LLM] Model: %s, Timeout: %ds", client.Model(), client.Timeout())

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[LLM] ERROR creating HTTP request: %v", err)
		return "", fmt.Errorf("cannot connect to LLM service: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.APIKey())
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: timeout}
	start := time.Now()

	log.Printf("[LLM] Sending HTTP request to LLM...")
	resp, err := httpClient.Do(req)
	if err != nil {
		elapsed := time.Since(start)
		log.Printf("[LLM] ERROR HTTP request failed after %v: %v", elapsed, err)
		return "", fmt.Errorf("cannot connect to LLM service: %v", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	log.Printf("[LLM] HTTP response received in %v: status=%d", elapsed, resp.StatusCode)

	// Read response body for logging
	var respBody bytes.Buffer
	_, err = respBody.ReadFrom(resp.Body)
	if err != nil {
		log.Printf("[LLM] ERROR reading response body: %v", err)
		return "", fmt.Errorf("failed to read LLM response: %v", err)
	}
	respBodyStr := respBody.String()
	log.Printf("[LLM] Response body (truncated): %.1000q", respBodyStr)

	if resp.StatusCode >= 400 {
		log.Printf("[LLM] ERROR LLM returned status %d: %s", resp.StatusCode, respBodyStr)
		return "", fmt.Errorf("LLM service error: status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(&respBody).Decode(&result); err != nil {
		log.Printf("[LLM] ERROR parsing JSON response: %v", err)
		return "", fmt.Errorf("failed to parse LLM response: %v", err)
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		log.Printf("[LLM] ERROR no choices in response: %#v", result)
		return "", fmt.Errorf("LLM returned no response")
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		log.Printf("[LLM] ERROR invalid choices[0] type: %#v", choices[0])
		return "", fmt.Errorf("invalid LLM response format")
	}

	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		log.Printf("[LLM] ERROR no message in first choice: %#v", firstChoice)
		return "", fmt.Errorf("invalid LLM response format")
	}

	content, ok := message["content"].(string)
	if !ok {
		log.Printf("[LLM] ERROR no content in message: %#v", message)
		return "", fmt.Errorf("LLM returned empty content")
	}

	log.Printf("[LLM] Success! Response length: %d chars", len(content))
	return content, nil
}

// extractDocument parses an LLM response to separate the conversational portion from
// any embedded requirements document. Documents are expected to be wrapped in --- delimiters:
//
//	---
//	<document content>
//	---
//
// The delimiter must appear on its own line (possibly with leading/trailing whitespace).
// Returns the conversational part (for the chat UI) and the extracted document content.
// If no delimited document is found, returns the full response as conversational part
// and an empty document string.
func extractDocument(response string) (conversational string, document string) {
	lines := strings.Split(response, "\n")

	openIdx := -1
	closeIdx := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if openIdx == -1 {
				openIdx = i
			} else {
				closeIdx = i
				break
			}
		}
	}

	if openIdx == -1 || closeIdx == -1 {
		// No pair of delimiters found — entire response is conversational
		return strings.TrimSpace(response), ""
	}

	// Reconstruct document content (lines between the two delimiters)
	docLines := lines[openIdx+1 : closeIdx]
	docContent := strings.TrimSpace(strings.Join(docLines, "\n"))

	// Reconstruct conversational parts (before opening and after closing delimiters)
	before := strings.TrimSpace(strings.Join(lines[:openIdx], "\n"))
	after := strings.TrimSpace(strings.Join(lines[closeIdx+1:], "\n"))

	// Combine conversational parts
	var convParts []string
	if before != "" {
		convParts = append(convParts, before)
	}
	if after != "" {
		convParts = append(convParts, after)
	}

	conversational = strings.Join(convParts, "\n\n")

	return conversational, docContent
}
