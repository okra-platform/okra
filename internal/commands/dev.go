package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/dev"
)

// Dev runs the development server with hot-reloading for OKRA services
func (c *Controller) Dev(ctx context.Context) error {
	// Load project configuration
	cfg, projectRoot, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	fmt.Printf("🚀 Starting OKRA development server for %s...\n", cfg.Name)
	fmt.Printf("📁 Project root: %s\n", projectRoot)
	fmt.Printf("📝 Schema: %s\n", filepath.Join(projectRoot, cfg.Schema))
	fmt.Printf("🔧 Language: %s\n", cfg.Language)
	
	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\n👋 Shutting down development server...")
		cancel()
	}()

	// Create and start the dev server
	server := dev.NewServer(cfg, projectRoot)
	if err := server.Start(ctx); err != nil {
		if err == context.Canceled {
			return nil
		}
		return fmt.Errorf("dev server error: %w", err)
	}

	return nil
}