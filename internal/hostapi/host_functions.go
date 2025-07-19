package hostapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// RunHostAPI is the single entry point for all host API calls
// It's exposed to WASM as "okra.run_host_api"
// The hostAPISet is retrieved from the context, set during WASM module instantiation
func RunHostAPI(ctx context.Context, requestJSON string) (string, error) {
	// Get the host API set for this service instance
	hostAPISet, ok := ctx.Value(hostAPISetKey{}).(HostAPISet)
	if !ok {
		return "", fmt.Errorf("host API set not found in context")
	}

	// Parse request
	var req HostAPIRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return "", fmt.Errorf("invalid request format: %w", err)
	}

	// Execute the method via the host API set
	// HostAPISet.Execute handles all cross-cutting concerns:
	// - API routing
	// - Telemetry (tracing and metrics)
	// - Policy enforcement
	// - Error handling
	result, err := hostAPISet.Execute(ctx, req.API, req.Method, req.Parameters)
	if err != nil {
		// Convert error to HostAPIError format
		var apiErr *HostAPIError
		if errors.As(err, &apiErr) {
			resp := HostAPIResponse{
				Success: false,
				Error:   apiErr,
			}
			respJSON, _ := json.Marshal(resp)
			return string(respJSON), nil
		}

		// Generic error
		resp := HostAPIResponse{
			Success: false,
			Error: &HostAPIError{
				Code:    ErrorCodeInternalError,
				Message: err.Error(),
			},
		}
		respJSON, _ := json.Marshal(resp)
		return string(respJSON), nil
	}

	// Return success response
	resp := HostAPIResponse{
		Success: true,
		Data:    result,
	}
	respJSON, _ := json.Marshal(resp)
	return string(respJSON), nil
}

// NextIterator is the entry point for iterator advancement
// It's exposed to WASM as "okra.next"
func NextIterator(ctx context.Context, requestJSON string) (string, error) {
	// Get the host API set for this service instance
	hostAPISet, ok := ctx.Value(hostAPISetKey{}).(HostAPISet)
	if !ok {
		return "", fmt.Errorf("host API set not found in context")
	}

	// Parse request
	var req NextRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return "", fmt.Errorf("invalid next request format: %w", err)
	}

	// Get next chunk from iterator
	data, hasMore, err := hostAPISet.NextIterator(ctx, req.IteratorID)
	if err != nil {
		// Convert error to NextResponse format
		var apiErr *HostAPIError
		if errors.As(err, &apiErr) {
			resp := NextResponse{
				Success: false,
				Error:   apiErr,
			}
			respJSON, _ := json.Marshal(resp)
			return string(respJSON), nil
		}

		// Generic error
		resp := NextResponse{
			Success: false,
			Error: &HostAPIError{
				Code:    ErrorCodeInternalError,
				Message: err.Error(),
			},
		}
		respJSON, _ := json.Marshal(resp)
		return string(respJSON), nil
	}

	// Return success response
	resp := NextResponse{
		Success: true,
		Data:    data,
		HasMore: hasMore,
	}
	respJSON, _ := json.Marshal(resp)
	return string(respJSON), nil
}

// RegisterHostAPI registers the unified host API function with Wazero
func RegisterHostAPI(ctx context.Context, runtime wazero.Runtime, hostAPISet HostAPISet) error {
	// Store the hostAPISet in a closure so it's accessible to the host function
	// This is safe because each WASM instance has its own hostAPISet

	// Get configuration for limits
	config := hostAPISet.Config()
	maxRequestSize := config.MaxRequestSize
	if maxRequestSize == 0 {
		maxRequestSize = DefaultMaxRequestSize
	}
	maxResponseSize := config.MaxResponseSize
	if maxResponseSize == 0 {
		maxResponseSize = DefaultMaxResponseSize
	}

	// Create the host module
	// Using "okra" namespace to clearly identify these as OKRA host functions
	// and avoid confusion with "env" which suggests environment variables
	builder := runtime.NewHostModuleBuilder("okra")

	// Helper function to handle WASM memory operations
	handleHostCall := func(ctx context.Context, module api.Module, stack []uint64, handler func(context.Context, string) (string, error)) {
		// Extract parameters from stack
		requestPtr := uint32(stack[0])
		requestLen := uint32(stack[1])

		// Validate request size
		if requestLen > uint32(maxRequestSize) {
			stack[0] = uint64(NullPointer)
			stack[1] = uint64(ZeroLength)
			return
		}

		// Read request from WASM memory
		requestBytes, ok := module.Memory().Read(requestPtr, requestLen)
		if !ok {
			stack[0] = uint64(NullPointer)
			stack[1] = uint64(ZeroLength)
			return
		}

		// Add hostAPISet to context
		ctx = context.WithValue(ctx, hostAPISetKey{}, hostAPISet)

		// Execute the handler
		response, err := handler(ctx, string(requestBytes))
		if err != nil {
			// This should not happen as handlers return error in the response
			stack[0] = uint64(NullPointer)
			stack[1] = uint64(ZeroLength)
			return
		}

		// Write response to WASM memory
		respBytes := []byte(response)

		// Validate response size
		if len(respBytes) > maxResponseSize {
			// Return error response for oversized response
			errResp := map[string]interface{}{
				"success": false,
				"error": map[string]string{
					"code":    ErrorCodeResponseTooLarge,
					"message": fmt.Sprintf("response size %d exceeds limit %d", len(respBytes), maxResponseSize),
				},
			}
			respBytes, _ = json.Marshal(errResp)
		}

		// Get the guest's allocate function
		allocate := module.ExportedFunction("allocate")
		if allocate == nil {
			stack[0] = uint64(NullPointer)
			stack[1] = uint64(ZeroLength)
			return
		}

		// Allocate memory in guest for response
		results, err := allocate.Call(ctx, uint64(len(respBytes)))
		if err != nil || len(results) == 0 {
			stack[0] = uint64(NullPointer)
			stack[1] = uint64(ZeroLength)
			return
		}
		respPtr := uint32(results[0])

		// Ensure we deallocate on any error
		deallocate := module.ExportedFunction("deallocate")
		deallocateOnError := func() {
			if deallocate != nil && respPtr != 0 {
				deallocate.Call(ctx, uint64(respPtr))
			}
		}

		// Write response to allocated memory
		if !module.Memory().Write(respPtr, respBytes) {
			deallocateOnError()
			stack[0] = uint64(NullPointer)
			stack[1] = uint64(ZeroLength)
			return
		}

		// Return pointer and length
		stack[0] = uint64(respPtr)
		stack[1] = uint64(len(respBytes))
	}

	// Register run_host_api function
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, module api.Module, stack []uint64) {
			handleHostCall(ctx, module, stack, RunHostAPI)
		}), []api.ValueType{
			api.ValueTypeI32, // requestPtr
			api.ValueTypeI32, // requestLen
		}, []api.ValueType{
			api.ValueTypeI32, // responsePtr
			api.ValueTypeI32, // responseLen
		}).
		Export("run_host_api")

	// Register the next function for iterator support
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, module api.Module, stack []uint64) {
			handleHostCall(ctx, module, stack, NextIterator)
		}), []api.ValueType{
			api.ValueTypeI32, // requestPtr
			api.ValueTypeI32, // requestLen
		}, []api.ValueType{
			api.ValueTypeI32, // responsePtr
			api.ValueTypeI32, // responseLen
		}).
		Export("next")

	// Instantiate the module with both functions
	_, err := builder.Instantiate(ctx)
	return err
}
