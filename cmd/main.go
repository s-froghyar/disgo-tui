package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/s-froghyar/disgo-tui/configs"
	"github.com/s-froghyar/disgo-tui/internal/client"
	"github.com/s-froghyar/disgo-tui/internal/tui"
)

// Build-time variable (set via -ldflags)
var version = "dev"

func main() {
	// Handle version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("Discogs TUI %s\n", version)
		fmt.Println("A terminal interface for your Discogs collection")
		fmt.Println("https://github.com/s-froghyar/disgo-tui")
		return
	}

	// Handle help flag
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Printf("Discogs TUI %s\n\n", version)
		fmt.Println("A terminal interface for your Discogs collection")
		fmt.Println("")
		fmt.Println("USAGE:")
		fmt.Println("  disgo-tui [FLAGS]")
		fmt.Println("")
		fmt.Println("FLAGS:")
		fmt.Println("  -h, --help     Show this help message")
		fmt.Println("  -v, --version  Show version information")
		fmt.Println("")
		fmt.Println("GETTING STARTED:")
		fmt.Println("  1. Run 'disgo-tui' to start the application")
		fmt.Println("  2. Authenticate with your Discogs account when prompted")
		fmt.Println("  3. Browse your collection using keyboard navigation")
		fmt.Println("")
		fmt.Println("NAVIGATION:")
		fmt.Println("  Ctrl+A        Focus menu")
		fmt.Println("  Ctrl+D        Focus grid")
		fmt.Println("  Arrow Keys    Navigate items")
		fmt.Println("  Enter         Open details")
		fmt.Println("  0,1,2         Switch views (Collection, Wishlist, Orders)")
		fmt.Println("  q             Quit")
		fmt.Println("")
		fmt.Println("For more information, visit:")
		fmt.Println("https://github.com/s-froghyar/disgo-tui")
		return
	}

	// Load configuration
	c, err := configs.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("🎵 Discogs TUI %s\n", version)

	// Create context with timeout for client initialization
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Create Discogs client
	httpClient, err := client.NewWithContext(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Discogs client: %v", err)
	}

	// Create and start TUI
	tuiApp := tui.New(httpClient, c)
	if err = tuiApp.Start(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}
}
