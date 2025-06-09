package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/s-froghyar/disgo-tui/configs"
	"github.com/s-froghyar/disgo-tui/internal/client"
	"github.com/s-froghyar/disgo-tui/internal/tui"
)

func main() {
	// Load configuration
	c, err := configs.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Check required environment variables
	requiredEnvVars := []string{
		"DISCOGS_API_CONSUMER_KEY",
		"DISCOGS_API_CONSUMER_SECRET",
		"LOCAL_PORT",
	}

	missing := false
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			log.Printf("Missing required environment variable: %s", envVar)
			missing = true
		}
	}

	if missing {
		log.Fatal(`
Required environment variables missing. Please set:
- DISCOGS_API_CONSUMER_KEY: Your Discogs API consumer key
- DISCOGS_API_CONSUMER_SECRET: Your Discogs API consumer secret  
- LOCAL_PORT: Port for OAuth callback (e.g., 8080)

You can get API credentials from: https://www.discogs.com/settings/developers`)
	}

	log.Println("Starting Discogs TUI...")
	log.Printf("OAuth callback will use port: %s", os.Getenv("LOCAL_PORT"))

	// Create context with timeout for client initialization
	// Give plenty of time for OAuth flow
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Create Discogs client with context support
	log.Println("Initializing Discogs client...")
	httpClient, err := client.NewWithContext(ctx)
	if err != nil {
		log.Fatalf("Failed to create Discogs client: %v", err)
	}

	log.Println("✓ Discogs client ready!")

	// Create TUI
	log.Println("Starting TUI application...")
	tuiApp := tui.New(httpClient, c)
	if err = tuiApp.Start(); err != nil {
		log.Fatalf("Failed to start TUI: %v", err)
	}
}
