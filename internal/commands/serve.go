package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/okra-platform/okra/internal/runtime"
	"github.com/okra-platform/okra/internal/serve"
	"github.com/rs/zerolog"
)

// Default ports for okra serve
const (
	defaultServicePort = 8080
	defaultAdminPort   = 8081
)

// ServeOptions contains options for the serve command
type ServeOptions struct {
	ServicePort int
	AdminPort   int
}

func (c *Controller) Serve(ctx context.Context, opts ...ServeOptions) error {
	// Use provided options or defaults
	servicePort := defaultServicePort
	adminPort := defaultAdminPort
	
	if len(opts) > 0 {
		if opts[0].ServicePort > 0 {
			servicePort = opts[0].ServicePort
		}
		if opts[0].AdminPort > 0 {
			adminPort = opts[0].AdminPort
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create logger
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	// Create runtime
	okraRuntime := runtime.NewOkraRuntime(logger)
	if err := okraRuntime.Start(ctx); err != nil {
		return fmt.Errorf("failed to start runtime: %w", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := okraRuntime.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("Error shutting down runtime: %v\n", err)
		}
	}()

	// Create gateways for service exposure
	connectGateway := runtime.NewConnectGateway()
	graphqlGateway := runtime.NewGraphQLGateway()

	// Create admin server
	adminServer := serve.NewAdminServer(okraRuntime, connectGateway, graphqlGateway)

	// Start both servers
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Start service gateway
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Printf("Starting service gateway on port %d...\n", servicePort)
		
		// Create HTTP server for both gateways
		mux := http.NewServeMux()
		mux.Handle("/connect/", connectGateway.Handler())
		mux.Handle("/graphql/", graphqlGateway.Handler())
		
		gatewayServer := &http.Server{
			Addr:    fmt.Sprintf(":%d", servicePort),
			Handler: mux,
		}
		
		if err := gatewayServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("service gateway error: %w", err)
		}
	}()

	// Start admin server
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Printf("Starting admin server on port %d...\n", adminPort)
		if err := adminServer.Start(ctx, adminPort); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("admin server error: %w", err)
		}
	}()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
		cancel()
	case err := <-errChan:
		fmt.Printf("Server error: %v\n", err)
		cancel()
		return err
	case <-ctx.Done():
		fmt.Println("Context cancelled, shutting down...")
	}

	// Wait for servers to stop
	wg.Wait()
	
	fmt.Println("Serve shutdown complete")
	return nil
}