package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/tochemey/goakt/v2/actors"
	"github.com/okra-platform/okra/internal/runtime/pb"
	"github.com/okra-platform/okra/internal/wasm"
	"google.golang.org/protobuf/types/known/durationpb"
)

// WASMActor is a GoAKT actor that executes WASM service methods
type WASMActor struct {
	// servicePackage contains the WASM module, schema, and config
	servicePackage *ServicePackage
	
	// workerPool manages WASM workers for execution
	workerPool wasm.WASMWorkerPool
	
	// minWorkers is the minimum number of workers in the pool
	minWorkers int
	
	// maxWorkers is the maximum number of workers in the pool
	maxWorkers int
	
	// ready indicates if the actor is ready to process requests
	ready bool
}

// NewWASMActor creates a new WASM actor with optional configuration
func NewWASMActor(servicePackage *ServicePackage, opts ...WASMActorOption) *WASMActor {
	actor := &WASMActor{
		servicePackage: servicePackage,
		minWorkers:     1,  // Default
		maxWorkers:     10, // Default
		ready:          false,
	}
	
	// Apply options
	for _, opt := range opts {
		opt(actor)
	}
	
	return actor
}

// PreStart initializes the actor before it starts receiving messages
func (a *WASMActor) PreStart(ctx context.Context) error {
	// Create worker pool if not already set (e.g., by tests)
	if a.workerPool == nil {
		poolConfig := wasm.WASMWorkerPoolConfig{
			MinWorkers: a.minWorkers,
			MaxWorkers: a.maxWorkers,
			Module:     a.servicePackage.Module,
		}
		
		pool, err := wasm.NewWASMWorkerPool(ctx, poolConfig)
		if err != nil {
			return fmt.Errorf("failed to create worker pool: %w", err)
		}
		
		a.workerPool = pool
	}
	
	a.ready = true
	return nil
}

// Receive handles incoming messages
func (a *WASMActor) Receive(ctx *actors.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *pb.ServiceRequest:
		a.handleServiceRequest(ctx, msg)
		
	case *pb.HealthCheck:
		a.handleHealthCheck(ctx, msg)
		
	default:
		// Log unhandled message
		ctx.Logger().Warnf("received unknown message type: %T", msg)
		ctx.Unhandled()
	}
}

// PostStop cleans up resources when the actor stops
func (a *WASMActor) PostStop(ctx context.Context) error {
	a.ready = false
	
	if a.workerPool != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		
		if err := a.workerPool.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shutdown worker pool: %w", err)
		}
	}
	
	return nil
}

// handleServiceRequest processes a service request
func (a *WASMActor) handleServiceRequest(ctx *actors.ReceiveContext, req *pb.ServiceRequest) {
	start := time.Now()
	
	// Check if actor is ready
	if !a.ready {
		response := pb.NewServiceResponse(req.GetId(), false)
		response.Error = pb.NewServiceError("INTERNAL_ERROR", "actor not ready")
		response.Duration = durationpb.New(time.Since(start))
		ctx.Response(response)
		return
	}
	
	// Validate the request
	if err := a.validateRequest(req); err != nil {
		response := pb.NewServiceResponse(req.GetId(), false)
		response.Error = pb.NewServiceError("VALIDATION_ERROR", err.Error())
		response.Duration = durationpb.New(time.Since(start))
		ctx.Response(response)
		return
	}
	
	// Create execution context with timeout
	execCtx := ctx.Context()
	if req.GetTimeout() != nil {
		timeout := req.GetTimeout().AsDuration()
		if timeout > 0 {
			var cancel context.CancelFunc
			execCtx, cancel = context.WithTimeout(execCtx, timeout)
			defer cancel()
		}
	}
	
	// Execute the method
	output, err := a.workerPool.Invoke(execCtx, req.GetMethod(), req.GetInput())
	if err != nil {
		ctx.Logger().Errorf("method execution failed: %v", err)
		response := pb.NewServiceResponse(req.GetId(), false)
		response.Error = pb.NewServiceError("EXECUTION_ERROR", err.Error())
		response.Duration = durationpb.New(time.Since(start))
		ctx.Response(response)
		return
	}
	
	// Send success response
	response := pb.NewServiceResponse(req.GetId(), true)
	response.Output = output
	response.Metadata = req.GetMetadata() // Copy request metadata
	response.Duration = durationpb.New(time.Since(start))
	
	ctx.Response(response)
}

// handleHealthCheck responds to health check requests
func (a *WASMActor) handleHealthCheck(ctx *actors.ReceiveContext, req *pb.HealthCheck) {
	response := &pb.HealthCheckResponse{
		Pong:  req.GetPing(),
		Ready: a.ready,
	}
	ctx.Response(response)
}

// validateRequest validates a service request against the schema
func (a *WASMActor) validateRequest(req *pb.ServiceRequest) error {
	method := req.GetMethod()
	if method == "" {
		return fmt.Errorf("method name is required")
	}
	
	// Check if method exists in schema
	methodDef, exists := a.servicePackage.GetMethod(method)
	if !exists {
		return fmt.Errorf("%w: %s", ErrMethodNotFound, method)
	}
	
	// Basic validation - ensure input is provided if method expects it
	if methodDef.InputType != "" && len(req.GetInput()) == 0 {
		return fmt.Errorf("%w: method %s requires input of type %s", 
			ErrInvalidInput, method, methodDef.InputType)
	}
	
	// TODO: Add more sophisticated validation based on schema types
	// This could include:
	// - Validating JSON structure matches expected type
	// - Checking required fields
	// - Validating enum values
	// - Type checking
	
	return nil
}

// Ensure WASMActor implements actors.Actor
var _ actors.Actor = (*WASMActor)(nil)