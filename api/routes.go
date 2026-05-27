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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"wavelength/internal/convert"
	"wavelength/internal/export"
	"wavelength/internal/llm"
	"wavelength/internal/mcp"
	topicpkg "wavelength/internal/topic"
)

// SetupRoutes registers all API routes on the given Fiber app.
func SetupRoutes(app *fiber.App, store topicpkg.TopicStore, client *llm.Client, dataDir string, mcpMgr *mcp.Manager) {
	// Static landing page
	app.Get("/", LandingPage)

	// Topic chat page (serves the UI, not JSON)
	app.Get("/topics/:id", TopicPage)

	// Health check is registered in main.go with voice status

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
		format := topicpkg.Format(strings.ToLower(c.Query("format", "markdown")))

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

			// Read file data (needed for both conversion and saving)
			fileData, err := io.ReadAll(src)
			if err != nil {
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
					"message": "failed to read uploaded file: " + err.Error(),
				})
			}

			// Convert to markdown
			conv := convert.New()
			markdown, err := conv.Convert(bytes.NewReader(fileData), file.Filename)
			if err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"message": "failed to convert document: " + err.Error(),
				})
			}

			// Determine format
			format, _ := convert.DetectFormat(file.Filename)

			// Save original file to disk
			attID := fmt.Sprintf("att-%s", uuid.New().String()[:8])
			attachmentsDir := filepath.Join(dataDir, "topics", topicID, "attachments")
			if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
					"message": "failed to create attachments directory: " + err.Error(),
				})
			}
			filePath := filepath.Join(attachmentsDir, attID+"."+filepath.Ext(file.Filename))
			if err := os.WriteFile(filePath, fileData, 0644); err != nil {
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
					"message": "failed to save uploaded file: " + err.Error(),
				})
			}

			// Create attachment record
			attachment := topicpkg.Attachment{
				ID:              attID,
				Filename:        file.Filename,
				Format:          string(format),
				Size:            file.Size,
				UploadedAt:      time.Now(),
				FilePath:        filepath.Join("attachments", attID+"."+filepath.Ext(file.Filename)),
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

	// Delete an attachment from a topic
	app.Delete("/api/topics/:id/attachments/:attachmentId", func(c *fiber.Ctx) error {
		topicID := c.Params("id")
		attachmentID := c.Params("attachmentId")

		topic := store.Get(topicID)
		if topic == nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"message": "topic not found",
			})
		}

		if !store.DeleteAttachment(topicID, attachmentID) {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"message": "attachment not found",
			})
		}

		return c.Status(http.StatusOK).JSON(fiber.Map{
			"message": "attachment deleted",
		})
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

		// Build prompt and call LLM with tool support
		prompt := buildConversationContext(topic, req.Content)
		llmMessages := []llm.Message{
			{Role: "system", Content: client.PersonaPrompt()},
			{Role: "user", Content: prompt},
		}

		// Build tools for the LLM
		topicDir := filepath.Join(dataDir, "topics", topicID)
		var toolDocUpdated bool
		tools := []*llm.Tool{
			llm.FileReadTool(topicDir),
			llm.WriteDocumentTool(topicDir, func(content string) {
				// Update the in-memory store when the LLM writes the document
				store.SetDocument(topicID, content)
				toolDocUpdated = true
				log.Printf("[DOC-TOOL] Tool wrote document for topic %q (%d bytes)", topicID, len(content))
			}),
		}

		// Add MCP tools if available
		if mcpMgr != nil {
			tools = append(tools, mcp.ToLLMTools(mcpMgr)...)
		}

		assistantResponse, err := client.CallWithTools(c.Context(), llmMessages, tools)
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

		// Extract any embedded requirements document from the LLM response (fallback)
		conversationalPart, extractedDoc := extractDocument(assistantResponse)

		// Document was updated either by the write_document tool or via delimiters
		documentUpdated := toolDocUpdated
		if !documentUpdated && extractedDoc != "" {
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

		// Build messages array for streaming (without system prompt — passed separately)
		llmMessages := make([]llm.Message, 0, len(topic.Messages))
		for _, msg := range topic.Messages {
			llmMessages = append(llmMessages, llm.Message{Role: msg.Role, Content: msg.Content})
		}
		llmMessages = append(llmMessages, llm.Message{Role: "user", Content: prompt})

		// Capture a valid context before spawning goroutine — Fiber's context
		// can become nil/cancelled once the request handler returns.
		// c.Context() returns *fasthttp.RequestCtx which implements context.Context.
		var streamCtx context.Context
		if fc := c.Context(); fc != nil {
			streamCtx = context.Context(fc)
		} else {
			streamCtx = context.Background()
		}

		// Set SSE headers
		c.Context().SetContentType("text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no")

		// Create a pipe: the HTTP response reads from the pipe reader, while the
		// goroutine writes to the pipe writer.  We use io.MultiWriter so every
		// write also goes into a capture buffer for document extraction — this
		// avoids the race condition of two goroutines competing to read from the
		// same pipe reader.
		pr, pw := io.Pipe()
		var captureBuffer strings.Builder
		tee := io.MultiWriter(pw, &captureBuffer)

		// Start streaming to client in a goroutine.
		// The "done" event is only sent AFTER all post-stream processing completes
		// (document extraction, follow-up tool calls, message persistence).
		// This ensures the frontend reloads the topic with up-to-date document state.
		go func() {
			defer pw.Close()

			// Send start event
			startEvent := map[string]interface{}{
				"type": "start",
			}
			json.NewEncoder(tee).Encode(startEvent)

			// Stream LLM response — tokens go to both HTTP response and capture buffer
			streamErr := client.StreamResponse(streamCtx, tee, client.PersonaPrompt(), llmMessages)
			if streamErr != nil {
				log.Printf("[LLM-STREAM] ERROR streaming response for topic %q: %v", topicID, streamErr)
				errEvent := map[string]interface{}{
					"type":    "error",
					"message": "The AI agent is temporarily unavailable. Your message has been saved.",
				}
				json.NewEncoder(tee).Encode(errEvent)
				// Send done even on error so frontend unblocks
				doneEvent := map[string]interface{}{
					"type":             "done",
					"document_updated": false,
				}
				json.NewEncoder(tee).Encode(doneEvent)
				return
			}

			// Process captured tokens for document extraction
			responseText := captureBuffer.String()
			var tokens []string
			scanner := bufio.NewScanner(strings.NewReader(responseText))
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}
				var event map[string]interface{}
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					log.Printf("[LLM-STREAM] WARN failed to parse captured event line for topic %q: %v (line: %.200q)", topicID, err, line)
					continue
				}
				if eventType, ok := event["type"].(string); ok && eventType == "token" {
					if content, ok := event["content"].(string); ok {
						tokens = append(tokens, content)
					}
				} else if eventType, ok := event["type"].(string); ok && eventType == "error" {
					if errMsg, ok := event["message"].(string); ok {
						log.Printf("[LLM-STREAM] ERROR event captured for topic %q: %s", topicID, errMsg)
					}
				}
			}

			assistantResponse := strings.Join(tokens, "")
			if assistantResponse == "" {
				log.Printf("[LLM-STREAM] WARN no assistant tokens captured for topic %q (buffer: %d bytes)", topicID, len(responseText))
				doneEvent := map[string]interface{}{
					"type":             "done",
					"document_updated": false,
				}
				json.NewEncoder(tee).Encode(doneEvent)
				return
			}

			log.Printf("[LLM-STREAM] Captured %d tokens (%d chars) for topic %q", len(tokens), len(assistantResponse), topicID)

			// Extract any embedded requirements document (delimiter-based fallback)
			conversationalPart, extractedDoc := extractDocument(assistantResponse)

			documentUpdated := false

			// Update document if extracted via delimiters
			if extractedDoc != "" {
				if store.SetDocument(topicID, extractedDoc) {
					documentUpdated = true
					log.Printf("[DOC] Updated requirement document for topic %q via delimiters (%d bytes)", topicID, len(extractedDoc))
				} else {
					log.Printf("[LLM-STREAM] WARN failed to save extracted document for topic %q", topicID)
				}
			}

			// If no document was extracted via delimiters, do a follow-up tool call
			// so the LLM can use write_document to persist the document.
			if !documentUpdated {
				topicDir := filepath.Join(dataDir, "topics", topicID)
				followUpTools := []*llm.Tool{
					llm.FileReadTool(topicDir),
					llm.WriteDocumentTool(topicDir, func(content string) {
						store.SetDocument(topicID, content)
						documentUpdated = true
						log.Printf("[DOC-TOOL] Follow-up tool wrote document for topic %q (%d bytes)", topicID, len(content))
					}),
				}

				// Add MCP tools if available
				if mcpMgr != nil {
					followUpTools = append(followUpTools, mcp.ToLLMTools(mcpMgr)...)
				}
				followUpMsg := fmt.Sprintf(
					"You just produced the following response in a requirements gathering session. "+
						"If this response contains or implies an updated requirements document, use the write_document tool to save it. "+
						"If the conversation does not warrant updating the document, simply reply 'no document update needed'.\n\n%s",
					assistantResponse,
				)
				followUpMessages := []llm.Message{
					{Role: "system", Content: client.PersonaPrompt()},
					{Role: "user", Content: followUpMsg},
				}
				followUpCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				_, err := client.CallWithTools(followUpCtx, followUpMessages, followUpTools)
				cancel()
				if err != nil {
					log.Printf("[LLM-STREAM] Follow-up tool call failed for topic %q: %v", topicID, err)
				}
			}

			// Save assistant response
			store.AddMessage(topicID, "assistant", conversationalPart)
			log.Printf("[LLM-STREAM] Saved assistant message for topic %q (%d chars conversational)", topicID, len(conversationalPart))

			// Send done event — only after all persistence is complete
			doneEvent := map[string]interface{}{
				"type":             "done",
				"document_updated": documentUpdated,
			}
			json.NewEncoder(tee).Encode(doneEvent)
			log.Printf("[LLM-STREAM] Stream complete for topic %q (document_updated=%v)", topicID, documentUpdated)
		}()

		// Stream the pipe to the HTTP response
		return c.SendStream(pr)
	})

	// Voice transcription endpoint
	app.Post("/api/voice/transcribe", func(c *fiber.Ctx) error {
		// Parse multipart form (max 20MB for audio)
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "failed to parse audio upload: " + err.Error(),
			})
		}

		files := form.File["audio"]
		if len(files) == 0 {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "no audio file provided. Use form field name 'audio'.",
			})
		}

		file := files[0]

		// Check file size (max 20MB)
		if file.Size > 20*1024*1024 {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "audio file too large (max 20MB)",
			})
		}

		// Open uploaded file
		src, err := file.Open()
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "failed to open audio file: " + err.Error(),
			})
		}
		defer src.Close()

		// Read audio data
		audioData, err := io.ReadAll(src)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "failed to read audio data: " + err.Error(),
			})
		}

		// Transcribe
		transcript, err := client.Transcribe(c.Context(), audioData)
		if err != nil {
			log.Printf("[VOICE] Transcription failed: %v", err)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "transcription failed: " + err.Error(),
			})
		}

		return c.Status(http.StatusOK).JSON(fiber.Map{
			"success": true,
			"text":    transcript,
		})
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
3. Provide an updated version of the document wrapped in the following delimiters if changes are needed:

=== REQUIREMENT DOCUMENT ===
<updated document content>
=== END REQUIREMENT DOCUMENT ===

Remember to maintain the document in markdown format and wrap any updated document in the delimiters above.`,
		topic.Name, description, document,
	)

	// Send re-evaluation prompt to the LLM
	reevalMessages := []llm.Message{
		{Role: "system", Content: client.PersonaPrompt()},
		{Role: "user", Content: reevalPrompt},
	}
	assistantResponse, err := client.Call(context.Background(), reevalMessages)
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

// (Re-evaluation now handled inline via client.Call) -- no separate function needed.

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
			buf.WriteString(fmt.Sprintf("\n[Document: %s (%s, %d bytes)]\n", att.Filename, att.Format, att.Size))
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
	} else {
		// No document exists yet — explicitly instruct the LLM to create one
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

		// First interaction — instruct the LLM to critically evaluate existing information
		// before responding to the user's message.
		if len(t.Messages) == 1 {
			buf.WriteString(fmt.Sprintf(
				"\nUser's latest message: %s\n\n"+
					"This is the first interaction for this topic. Before responding to the user, "+
					"critically evaluate the current requirement document and all available information above. "+
					"Identify gaps, inconsistencies, or missing sections. "+
					"Provide an updated version of the document wrapped in the following delimiters if improvements are needed:\n"+
					"=== REQUIREMENT DOCUMENT ===\n"+
					"<updated document content>\n"+
					"=== END REQUIREMENT DOCUMENT ===\n\n"+
					"Then address the user's message as a business analyst conducting requirements gathering.",
				userMessage,
			))
			return buf.String()
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

// (Non-streaming response now handled inline via client.Call) -- no separate function needed.

// extractDocument parses an LLM response to separate the conversational portion from
// any embedded requirements document. Documents are expected to be wrapped in unique
// delimiters:
//
//	=== REQUIREMENT DOCUMENT ===
//	<document content>
//	=== END REQUIREMENT DOCUMENT ===
//
// The delimiters must appear on their own lines (possibly with leading/trailing whitespace).
// Returns the conversational part (for the chat UI) and the extracted document content.
// If no delimited document is found, returns the full response as conversational part
// and an empty document string.
const docDelimOpen = "=== REQUIREMENT DOCUMENT ==="
const docDelimClose = "=== END REQUIREMENT DOCUMENT ==="

func extractDocument(response string) (conversational string, document string) {
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
