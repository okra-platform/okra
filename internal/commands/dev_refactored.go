package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/dev"
)

// DevDependencies for the dev command
type DevDependencies struct {
	ConfigLoader   ConfigLoader
	ServerFactory  DevServerFactory
	SignalNotifier SignalNotifier
	Output         Output
}

// Interfaces for dependency injection
type ConfigLoader interface {
	LoadConfig() (*config.Config, string, error)
}

type DevServerFactory interface {
	NewServer(cfg *config.Config, projectRoot string) DevServer
}

type DevServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Default implementations
type defaultConfigLoader struct{}

func (l *defaultConfigLoader) LoadConfig() (*config.Config, string, error) {
	return config.LoadConfig()
}

type defaultDevServerFactory struct{}

func (f *defaultDevServerFactory) NewServer(cfg *config.Config, projectRoot string) DevServer {
	return dev.NewServer(cfg, projectRoot)
}

// DevCommand encapsulates the dev logic with injected dependencies
type DevCommand struct {
	deps DevDependencies
}

// NewDevCommand creates a new dev command with default dependencies
func NewDevCommand() *DevCommand {
	return &DevCommand{
		deps: DevDependencies{
			ConfigLoader:   &defaultConfigLoader{},
			ServerFactory:  &defaultDevServerFactory{},
			SignalNotifier: &defaultSignalNotifier{},
			Output:         &defaultOutput{},
		},
	}
}

// WithDependencies allows injecting custom dependencies for testing
func (dc *DevCommand) WithDependencies(deps DevDependencies) *DevCommand {
	dc.deps = deps
	return dc
}

// Execute runs the dev command
func (dc *DevCommand) Execute(ctx context.Context) error {
	// Load project configuration
	cfg, projectRoot, err := dc.deps.ConfigLoader.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	dc.deps.Output.Printf("🚀 Starting OKRA development server for %s...\n", cfg.Name)
	dc.deps.Output.Printf("📁 Project root: %s\n", projectRoot)
	dc.deps.Output.Printf("📝 Schema: %s\n", filepath.Join(projectRoot, cfg.Schema))
	dc.deps.Output.Printf("🔧 Language: %s\n", cfg.Language)

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	dc.deps.SignalNotifier.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer dc.deps.SignalNotifier.Stop(sigChan)
	
	// Start signal handler
	go func() {
		select {
		case <-sigChan:
			dc.deps.Output.Println("\n\n👋 Shutting down development server...")
			cancel()
		case <-ctx.Done():
			// Context cancelled, no need to do anything
		}
	}()

	// Create and start the dev server
	server := dc.deps.ServerFactory.NewServer(cfg, projectRoot)
	if err := server.Start(ctx); err != nil {
		if err == context.Canceled {
			return nil
		}
		return fmt.Errorf("dev server error: %w", err)
	}

	return nil
}

// Update the Controller's Dev method to use the refactored command
func (c *Controller) DevRefactored(ctx context.Context) error {
	cmd := NewDevCommand()
	return cmd.Execute(ctx)
}