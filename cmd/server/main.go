package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"wavelength/api"
	"wavelength/internal/config"
	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

func main() {
	cfgPath := flag.String("config", "configs/config.json", "Path to the JSON configuration file")
	flag.Parse()

	// Load configuration from the single JSON file
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate all required configuration fields
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Create LLM client
	client := llm.NewClient(cfg)

	// Check LLM connectivity at startup — warn but don't block
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.CheckConnectivity(ctx); err != nil {
		log.Printf("WARNING: LLM connectivity check failed: %v", err)
		log.Println("The application will start, but the AI agent will be unavailable until the LLM service is reachable.")
	} else {
		log.Println("LLM connectivity check passed.")
	}

	// Create file-backed topic store and load persisted topics
	store := topic.NewFileStore(cfg.DataDir)
	if err := store.LoadAll(); err != nil {
		log.Fatalf("Failed to load topics from disk: %v", err)
	}
	log.Printf("Loaded %d topics from %s", len(store.List()), cfg.DataDir)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: false,
	})

	// Register all routes
	api.SetupRoutes(app, store, client)

	// Persist topics on shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down... persisting topics to disk.")
		if err := store.SaveAll(); err != nil {
			log.Printf("Failed to persist topics on shutdown: %v", err)
		}
		os.Exit(0)
	}()

	// Persist topics periodically
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := store.SaveAll(); err != nil {
				log.Printf("Failed to persist topics: %v", err)
			}
		}
	}()

	// Start the server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Wavelength server starting on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
