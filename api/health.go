package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/llm"
)

// HealthHandler returns a Fiber handler that reports application and LLM health status.
func HealthHandler(client *llm.Client, voiceAvailable bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		health := map[string]interface{}{
			"status": "running",
			"llm": map[string]interface{}{
				"status": "checking",
			},
			"voice": map[string]interface{}{
				"status": "unavailable",
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

		// Voice status
		if voiceAvailable {
			health["voice"].(map[string]interface{})["status"] = "available"
		} else {
			health["voice"].(map[string]interface{})["status"] = "unavailable"
			health["voice"].(map[string]interface{})["reason"] = "whisper endpoint not available or disabled"
		}

		return c.Status(http.StatusOK).JSON(health)
	}
}
