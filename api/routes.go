package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// SetupRoutes registers all API routes on the given Fiber app.
func SetupRoutes(app *fiber.App, store topic.TopicStore, client *llm.Client) {
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

	// Submit message to topic conversation
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

		// Save assistant response
		store.AddMessage(topicID, "assistant", assistantResponse)

		return c.Status(http.StatusOK).JSON(fiber.Map{
			"success": true,
			"message": fiber.Map{
				"role":      "assistant",
				"content":   assistantResponse,
				"timestamp": time.Now(),
			},
		})
	})
}

// generateResponse calls the LLM to generate a response based on the conversation history.
func generateResponse(client *llm.Client, t *topic.Topic, userMessage string) (string, error) {
	log.Printf("[LLM] === Request for topic %q ===", t.ID)

	timeout := time.Duration(client.Timeout()) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Build conversation context
	var buf bytes.Buffer
	log.Printf("[LLM] Building conversation context from %d messages", len(t.Messages))
	for i, msg := range t.Messages {
		preview := msg.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		log.Printf("[LLM]   Message %d (%s): %s", i+1, msg.Role, preview)
		fmt.Fprintf(&buf, "%s: %s\n", msg.Role, msg.Content)
	}

	// Include the high-level requirement description in the prompt
	description := t.Description
	if description == "" {
		description = "(no description provided)"
	}

	// For now, do a simple HTTP POST to the LLM endpoint with the conversation
	prompt := fmt.Sprintf("Topic: %s\nHigh-level requirement: %s\n\nConversation context:\n%s\n\nUser's latest message: %s\n\nPlease respond as a business analyst conducting requirements gathering.", t.Name, description, buf.String(), userMessage)

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
