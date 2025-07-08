package runtime

import (
	"context"
)

// Runtime manages the actor system and service deployments
type Runtime interface {
	// Start initializes the runtime and starts the actor system
	Start(ctx context.Context) error
	
	// Deploy deploys a service package to the runtime
	// Returns the actor ID (fully qualified service name)
	Deploy(ctx context.Context, pkg *ServicePackage) (string, error)
	
	// Undeploy removes a service from the runtime
	Undeploy(ctx context.Context, actorID string) error
	
	// IsDeployed checks if a service is deployed
	IsDeployed(actorID string) bool
	
	// Shutdown gracefully shuts down the runtime and all actors
	Shutdown(ctx context.Context) error
}