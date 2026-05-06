package api

import (
	"embed"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

//go:embed static/*
var staticFiles embed.FS

// LandingPage serves the main Wavelength interface.
func LandingPage(c *fiber.Ctx) error {
	index, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("Failed to load application")
	}
	return c.Type("html").Send(index)
}

// TopicPage serves the chat interface for a single topic.
func TopicPage(c *fiber.Ctx) error {
	page, err := staticFiles.ReadFile("static/topic.html")
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("Failed to load application")
	}
	return c.Type("html").Send(page)
}
