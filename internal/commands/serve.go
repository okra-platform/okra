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

// Dependencies for the serve command
type ServeDependencies struct {
	RuntimeFactory    RuntimeFactory
	GatewayFactory    GatewayFactory
	AdminServerFactory AdminServerFactory
	HTTPServerFactory HTTPServerFactory
	SignalNotifier    SignalNotifier
	Logger            zerolog.Logger
	Output            Output
}

// Interfaces for dependency injection
type RuntimeFactory interface {
	NewRuntime(logger zerolog.Logger) runtime.Runtime
}

type GatewayFactory interface {
	NewConnectGateway() runtime.ConnectGateway
	NewGraphQLGateway() runtime.GraphQLGateway
}

type AdminServerFactory interface {
	NewAdminServer(runtime runtime.Runtime, connectGateway runtime.ConnectGateway, graphqlGateway runtime.GraphQLGateway) AdminServer
}

type AdminServer interface {
	Start(ctx context.Context, port int) error
}

type HTTPServerFactory interface {
	NewHTTPServer(addr string, handler http.Handler) HTTPServer
}

type HTTPServer interface {
	ListenAndServe() error
}

type SignalNotifier interface {
	Notify(c chan<- os.Signal, sig ...os.Signal)
	Stop(c chan<- os.Signal)
}

type Output interface {
	Printf(format string, a ...interface{})
	Println(a ...interface{})
}

// Default implementations
type defaultRuntimeFactory struct{}

func (f *defaultRuntimeFactory) NewRuntime(logger zerolog.Logger) runtime.Runtime {
	return runtime.NewOkraRuntime(logger)
}

type defaultGatewayFactory struct{}

func (f *defaultGatewayFactory) NewConnectGateway() runtime.ConnectGateway {
	return runtime.NewConnectGateway()
}

func (f *defaultGatewayFactory) NewGraphQLGateway() runtime.GraphQLGateway {
	return runtime.NewGraphQLGateway()
}

type defaultAdminServerFactory struct{}

func (f *defaultAdminServerFactory) NewAdminServer(runtime runtime.Runtime, connectGateway runtime.ConnectGateway, graphqlGateway runtime.GraphQLGateway) AdminServer {
	return serve.NewAdminServer(runtime, connectGateway, graphqlGateway)
}

type defaultHTTPServerFactory struct{}

func (f *defaultHTTPServerFactory) NewHTTPServer(addr string, handler http.Handler) HTTPServer {
	return &httpServerWrapper{
		Server: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}
}

type httpServerWrapper struct {
	*http.Server
}

func (s *httpServerWrapper) ListenAndServe() error {
	return s.Server.ListenAndServe()
}

type defaultSignalNotifier struct{}

func (n *defaultSignalNotifier) Notify(c chan<- os.Signal, sig ...os.Signal) {
	signal.Notify(c, sig...)
}

func (n *defaultSignalNotifier) Stop(c chan<- os.Signal) {
	signal.Stop(c)
}

type defaultOutput struct{}

func (o *defaultOutput) Printf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

func (o *defaultOutput) Println(a ...interface{}) {
	fmt.Println(a...)
}

// ServeCommand encapsulates the serve logic with injected dependencies
type ServeCommand struct {
	deps ServeDependencies
}

// NewServeCommand creates a new serve command with default dependencies
func NewServeCommand() *ServeCommand {
	return &ServeCommand{
		deps: ServeDependencies{
			RuntimeFactory:     &defaultRuntimeFactory{},
			GatewayFactory:     &defaultGatewayFactory{},
			AdminServerFactory: &defaultAdminServerFactory{},
			HTTPServerFactory:  &defaultHTTPServerFactory{},
			SignalNotifier:     &defaultSignalNotifier{},
			Logger:             zerolog.New(os.Stderr).With().Timestamp().Logger(),
			Output:             &defaultOutput{},
		},
	}
}

// WithDependencies allows injecting custom dependencies for testing
func (sc *ServeCommand) WithDependencies(deps ServeDependencies) *ServeCommand {
	sc.deps = deps
	return sc
}

// Execute runs the serve command with the given options
func (sc *ServeCommand) Execute(ctx context.Context, opts ServeOptions) error {
	// Use provided options or defaults
	servicePort := opts.ServicePort
	if servicePort == 0 {
		servicePort = defaultServicePort
	}
	adminPort := opts.AdminPort
	if adminPort == 0 {
		adminPort = defaultAdminPort
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create runtime
	okraRuntime := sc.deps.RuntimeFactory.NewRuntime(sc.deps.Logger)
	if err := okraRuntime.Start(ctx); err != nil {
		return fmt.Errorf("failed to start runtime: %w", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := okraRuntime.Shutdown(shutdownCtx); err != nil {
			sc.deps.Output.Printf("Error shutting down runtime: %v\n", err)
		}
	}()

	// Create gateways for service exposure
	connectGateway := sc.deps.GatewayFactory.NewConnectGateway()
	graphqlGateway := sc.deps.GatewayFactory.NewGraphQLGateway()

	// Create admin server
	adminServer := sc.deps.AdminServerFactory.NewAdminServer(okraRuntime, connectGateway, graphqlGateway)

	// Start both servers
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Start service gateway
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc.deps.Output.Printf("Starting service gateway on port %d...\n", servicePort)

		// Create HTTP server for both gateways
		mux := http.NewServeMux()
		mux.Handle("/connect/", connectGateway.Handler())
		mux.Handle("/graphql/", graphqlGateway.Handler())

		gatewayServer := sc.deps.HTTPServerFactory.NewHTTPServer(
			fmt.Sprintf(":%d", servicePort),
			mux,
		)

		if err := gatewayServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("service gateway error: %w", err)
		}
	}()

	// Start admin server
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc.deps.Output.Printf("Starting admin server on port %d...\n", adminPort)
		if err := adminServer.Start(ctx, adminPort); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("admin server error: %w", err)
		}
	}()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	sc.deps.SignalNotifier.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer sc.deps.SignalNotifier.Stop(sigChan)

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		sc.deps.Output.Printf("\nReceived signal %v, shutting down...\n", sig)
		cancel()
	case err := <-errChan:
		sc.deps.Output.Printf("Server error: %v\n", err)
		cancel()
		return err
	case <-ctx.Done():
		sc.deps.Output.Println("Context cancelled, shutting down...")
	}

	// Wait for servers to stop
	wg.Wait()

	sc.deps.Output.Println("Serve shutdown complete")
	return nil
}

// Controller's Serve method 
func (c *Controller) Serve(ctx context.Context, opts ...ServeOptions) error {
	// Convert to single ServeOptions
	serveOpts := ServeOptions{
		ServicePort: defaultServicePort,
		AdminPort:   defaultAdminPort,
	}
	
	if len(opts) > 0 {
		if opts[0].ServicePort > 0 {
			serveOpts.ServicePort = opts[0].ServicePort
		}
		if opts[0].AdminPort > 0 {
			serveOpts.AdminPort = opts[0].AdminPort
		}
	}
	
	cmd := NewServeCommand()
	return cmd.Execute(ctx, serveOpts)
}