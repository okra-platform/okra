package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"github.com/tochemey/goakt/v2/actors"
)

// OkraRuntime is the default implementation of Runtime
type OkraRuntime struct {
	// actorSystem is the GoAKT actor system
	actorSystem actors.ActorSystem

	// deployedActors tracks deployed service actors
	deployedActors map[string]*actors.PID
	mu             sync.RWMutex

	// logger for runtime operations
	logger zerolog.Logger

	// started indicates if the runtime has been started
	started bool
}

// NewOkraRuntime creates a new runtime instance
func NewOkraRuntime(logger zerolog.Logger) *OkraRuntime {
	return &OkraRuntime{
		deployedActors: make(map[string]*actors.PID),
		logger:         logger.With().Str("component", "runtime").Logger(),
		started:        false,
	}
}

// Start initializes the runtime and starts the actor system
func (r *OkraRuntime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return fmt.Errorf("runtime already started")
	}

	// Create actor system
	// Note: GoAKT uses its default logger. We track operations separately with zerolog
	actorSystem, err := actors.NewActorSystem("okra-runtime")
	if err != nil {
		return fmt.Errorf("failed to create actor system: %w", err)
	}

	// Start the actor system
	if err := actorSystem.Start(ctx); err != nil {
		return fmt.Errorf("failed to start actor system: %w", err)
	}

	r.actorSystem = actorSystem
	r.started = true

	r.logger.Info().Msg("runtime started successfully")
	return nil
}

// Deploy deploys a service package to the runtime
func (r *OkraRuntime) Deploy(ctx context.Context, pkg *ServicePackage) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return "", fmt.Errorf("runtime not started")
	}

	if pkg == nil {
		return "", fmt.Errorf("service package cannot be nil")
	}

	// Generate actor ID from service package
	// Format: namespace.service.version
	actorID := r.generateActorID(pkg)

	// Check if already deployed
	if _, exists := r.deployedActors[actorID]; exists {
		return "", fmt.Errorf("service %s already deployed", actorID)
	}

	// Create WASM actor
	actor := NewWASMActor(pkg)

	// Spawn the actor
	pid, err := r.actorSystem.Spawn(ctx, actorID, actor)
	if err != nil {
		return "", fmt.Errorf("failed to spawn actor %s: %w", actorID, err)
	}

	// Track deployed actor
	r.deployedActors[actorID] = pid

	r.logger.Info().
		Str("actor_id", actorID).
		Str("service", pkg.ServiceName).
		Msg("service deployed successfully")

	return actorID, nil
}

// Undeploy removes a service from the runtime
func (r *OkraRuntime) Undeploy(ctx context.Context, actorID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return fmt.Errorf("runtime not started")
	}

	pid, exists := r.deployedActors[actorID]
	if !exists {
		return fmt.Errorf("service %s not deployed", actorID)
	}

	// Shutdown the actor
	if err := pid.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown actor %s: %w", actorID, err)
	}

	// Remove from tracking
	delete(r.deployedActors, actorID)

	r.logger.Info().
		Str("actor_id", actorID).
		Msg("service undeployed successfully")

	return nil
}

// IsDeployed checks if a service is deployed
func (r *OkraRuntime) IsDeployed(actorID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.deployedActors[actorID]
	return exists
}

// Shutdown gracefully shuts down the runtime and all actors
func (r *OkraRuntime) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return fmt.Errorf("runtime not started")
	}

	// Log shutdown initiation
	r.logger.Info().
		Int("deployed_actors", len(r.deployedActors)).
		Msg("shutting down runtime")

	// Shutdown all deployed actors first
	var shutdownErrors []error
	for actorID, pid := range r.deployedActors {
		if err := pid.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors,
				fmt.Errorf("failed to shutdown actor %s: %w", actorID, err))
			r.logger.Error().
				Err(err).
				Str("actor_id", actorID).
				Msg("failed to shutdown actor")
		}
	}

	// Clear deployed actors
	r.deployedActors = make(map[string]*actors.PID)

	// Stop the actor system
	if err := r.actorSystem.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop actor system: %w", err)
	}

	r.started = false
	r.logger.Info().Msg("runtime shutdown complete")

	// Return first error if any occurred during actor shutdown
	if len(shutdownErrors) > 0 {
		return shutdownErrors[0]
	}

	return nil
}

// generateActorID creates a fully qualified actor ID from the service package.
// The format is: namespace.ServiceName.version
// Example: "myapp.GreeterService.v1"
// This ID is used for actor registration and must match the service name
// used in the ConnectGateway for proper routing.
func (r *OkraRuntime) generateActorID(pkg *ServicePackage) string {
	// Extract namespace from schema metadata
	namespace := "default"
	if pkg.Schema != nil && pkg.Schema.Meta.Namespace != "" {
		namespace = pkg.Schema.Meta.Namespace
	}

	// Extract version from schema metadata
	version := "v1"
	if pkg.Schema != nil && pkg.Schema.Meta.Version != "" {
		version = pkg.Schema.Meta.Version
	}

	// Format: namespace.service.version
	return fmt.Sprintf("%s.%s.%s", namespace, pkg.ServiceName, version)
}

// GetActorPID returns the PID for a given actor ID
func (o *OkraRuntime) GetActorPID(actorID string) *actors.PID {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if pid, exists := o.deployedActors[actorID]; exists {
		return pid
	}
	return nil
}

// Ensure OkraRuntime implements Runtime interface
var _ Runtime = (*OkraRuntime)(nil)
