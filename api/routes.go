package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// SetupRoutes registers all API routes on the given Fiber app.
func SetupRoutes(app *fiber.App, store *topic.Store, client *llm.Client) {
	// Static landing page
	app.Get("/", LandingPage)

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

	// Submit message to topic conversation
	app.Post("/api/topics/:id/messages", func(c *fiber.Ctx) error {
		topicID := c.Params("id")
		topic := store.Get(topicID)
		if topic == nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"message": "topic not found",
			})
		}

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

		// Always save the user's message first
		store.AddMessage(topicID, "user", req.Content)

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build conversation context
	var buf bytes.Buffer
	for _, msg := range t.Messages {
		if msg.Role == "user" {
			fmt.Fprintf(&buf, "User: %s\n", msg.Content)
		} else {
			fmt.Fprintf(&buf, "Assistant: %s\n", msg.Content)
		}
	}

	// For now, do a simple HTTP POST to the LLM endpoint with the conversation
	prompt := fmt.Sprintf("Conversation context:\n%s\n\nUser's latest message: %s\n\nPlease respond as a business analyst conducting requirements gathering.", buf.String(), userMessage)

	payload := map[string]interface{}{
		"model": client.Model(),
		"messages": []map[string]string{
			{"role": "system", "content": t.Document}, // will be persona prompt
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to prepare LLM request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", client.Endpoint()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("cannot connect to LLM service: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.APIKey())
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 30 * time.Second}
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

	return content, nil
}
