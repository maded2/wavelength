package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/llm"
)

// HealthHandler returns a Fiber handler that reports application and LLM health status.
func HealthHandler(client *llm.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		health := map[string]interface{}{
			"status": "running",
			"llm": map[string]interface{}{
				"status": "checking",
			},
		}

		// Perform a quick connectivity check
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err := client.CheckConnectivity(ctx)
		if err != nil {
			health["llm"].(map[string]interface{})["status"] = "unavailable"
			health["llm"].(map[string]interface{})["reason"] = err.Error()
		} else {
			health["llm"].(map[string]interface{})["status"] = "available"
		}

		return c.Status(http.StatusOK).JSON(health)
	}
}
