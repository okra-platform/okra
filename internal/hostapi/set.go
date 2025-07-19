package hostapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HostAPISet contains all host API instances for a specific service
type HostAPISet interface {
	// Get retrieves a specific host API instance
	Get(name string) (HostAPI, bool)

	// Execute routes a request to the appropriate host API
	Execute(ctx context.Context, apiName, method string, parameters json.RawMessage) (json.RawMessage, error)

	// NextIterator advances an iterator and returns the next chunk
	NextIterator(ctx context.Context, iteratorID string) (json.RawMessage, bool, error)

	// CloseIterator cleans up iterator resources
	CloseIterator(ctx context.Context, iteratorID string) error

	// CleanupStaleIterators removes iterators older than the timeout
	CleanupStaleIterators() int

	// Config returns the configuration for this host API set
	Config() HostAPIConfig

	// Close cleans up all host API resources
	// Should be called after the WASM instance has terminated
	Close() error
}

// defaultHostAPISet is the concrete implementation of HostAPISet
// Each WASM instance gets its own HostAPISet, but we still need synchronization
// because host-side Go code may have concurrent access patterns
type defaultHostAPISet struct {
	apis      map[string]HostAPI
	iterators map[string]*iteratorInfo // Active iterators
	config    HostAPIConfig
	closed    bool // Defensive: tracks if Close() has been called
	mu        sync.RWMutex
}

// Compile-time interface compliance checks
var (
	_ HostAPISet      = (*defaultHostAPISet)(nil)
	_ HostAPIRegistry = (*defaultHostAPIRegistry)(nil)
)

// Get retrieves a specific host API instance
func (s *defaultHostAPISet) Get(name string) (HostAPI, bool) {
	api, ok := s.apis[name]
	return api, ok
}

// Execute routes a request to the appropriate host API with cross-cutting concerns
func (s *defaultHostAPISet) Execute(ctx context.Context, apiName, method string, parameters json.RawMessage) (json.RawMessage, error) {
	// Defensive check: ensure we're not using a closed HostAPISet
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, &HostAPIError{
			Code:    ErrorCodeHostAPISetClosed,
			Message: "HostAPISet has been closed - this indicates improper lifecycle management",
			Details: "Execute called after Close()",
		}
	}
	s.mu.RUnlock()

	// Get the API instance
	api, ok := s.apis[apiName]
	if !ok {
		return nil, &HostAPIError{
			Code:    ErrorCodeAPINotFound,
			Message: fmt.Sprintf("host API %s not found", apiName),
		}
	}

	// Start telemetry span
	ctx, span := s.config.Tracer.Start(ctx, fmt.Sprintf("host.%s.%s", apiName, method))
	defer span.End()

	// Policy check
	serviceInfo, _ := ctx.Value(serviceInfoKey{}).(ServiceInfo)
	decision, err := s.config.PolicyEngine.Evaluate(ctx, PolicyCheck{
		Service: serviceInfo.Name,
		Request: HostAPIRequest{
			API:        apiName,
			Method:     method,
			Parameters: parameters,
			Metadata:   RequestMetadata{ServiceInfo: serviceInfo},
		},
		Context: make(map[string]interface{}),
	})

	if err != nil {
		span.RecordError(err)
		return nil, &HostAPIError{
			Code:    ErrorCodePolicyError,
			Message: fmt.Sprintf("policy evaluation failed: %v", err),
		}
	}

	if !decision.Allowed {
		return nil, &HostAPIError{
			Code:    ErrorCodePolicyDenied,
			Message: decision.Reason,
		}
	}

	// Execute the API method
	start := time.Now()
	var result json.RawMessage
	var executeErr error

	// Check if this API supports streaming and if this is a streaming method
	if streamingAPI, ok := api.(StreamingHostAPI); ok {
		// Check method metadata to see if this is a streaming method
		// In practice, we'd look this up from the factory's Methods()
		var iterator Iterator
		result, iterator, executeErr = streamingAPI.ExecuteStreaming(ctx, method, parameters)

		// If we got an iterator, register it
		if executeErr == nil && iterator != nil {
			var streamResp StreamingResponse
			if json.Unmarshal(result, &streamResp) == nil && streamResp.IteratorID != "" {
				s.mu.Lock()

				// Check iterator limit
				maxIterators := s.config.MaxIteratorsPerService
				if maxIterators == 0 {
					maxIterators = DefaultMaxIteratorsPerService
				}
				if len(s.iterators) >= maxIterators {
					s.mu.Unlock()
					iterator.Close() // Clean up the iterator
					return nil, &HostAPIError{
						Code:    ErrorCodeIteratorLimitExceeded,
						Message: fmt.Sprintf("maximum concurrent iterators (%d) exceeded", maxIterators),
					}
				}

				s.iterators[streamResp.IteratorID] = &iteratorInfo{
					iterator:  iterator,
					apiName:   apiName,
					method:    method,
					createdAt: time.Now(),
				}
				s.mu.Unlock()
			}
		}
	} else {
		// Regular non-streaming execution
		result, executeErr = api.Execute(ctx, method, parameters)
	}
	duration := time.Since(start)

	// Record metrics
	attrs := []attribute.KeyValue{
		attribute.String("api", apiName),
		attribute.String("method", method),
		attribute.Bool("success", executeErr == nil),
	}

	callCounter, _ := s.config.Meter.Int64Counter("host_api_calls")
	callCounter.Add(ctx, 1, metric.WithAttributes(attrs...))

	durationHistogram, _ := s.config.Meter.Float64Histogram("host_api_duration_ms")
	durationHistogram.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs[:2]...))

	if executeErr != nil {
		span.RecordError(executeErr)
		if s.config.Logger != nil {
			s.config.Logger.Error("host API call failed",
				"api", apiName,
				"method", method,
				"error", executeErr,
				"duration_ms", duration.Milliseconds(),
			)
		}
		return nil, executeErr
	}

	return result, nil
}

// NextIterator advances an iterator and returns the next chunk
func (s *defaultHostAPISet) NextIterator(ctx context.Context, iteratorID string) (json.RawMessage, bool, error) {
	// Defensive check with proper locking
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, false, &HostAPIError{
			Code:    ErrorCodeHostAPISetClosed,
			Message: "HostAPISet has been closed",
		}
	}

	// Get iterator while holding read lock
	info, ok := s.iterators[iteratorID]
	s.mu.RUnlock()

	if !ok {
		return nil, false, &HostAPIError{
			Code:    ErrorCodeIteratorNotFound,
			Message: fmt.Sprintf("iterator %s not found", iteratorID),
		}
	}

	// Validate iterator belongs to calling service (from context)
	if serviceInfo, ok := ctx.Value(serviceInfoKey{}).(ServiceInfo); ok {
		// In practice, we'd store service info with the iterator and validate here
		// For now, we'll add a TODO
		// TODO: Add service validation to prevent cross-service iterator access
		_ = serviceInfo
	}

	// Start telemetry span
	ctx, span := s.config.Tracer.Start(ctx, fmt.Sprintf("host.%s.%s.next", info.apiName, info.method))
	defer span.End()

	// Get next chunk
	start := time.Now()
	data, hasMore, err := info.iterator.Next(ctx)
	duration := time.Since(start)

	// Record metrics
	attrs := []attribute.KeyValue{
		attribute.String("api", info.apiName),
		attribute.String("method", info.method),
		attribute.Bool("success", err == nil),
		attribute.Bool("has_more", hasMore),
	}

	iteratorCounter, _ := s.config.Meter.Int64Counter("host_api_iterator_calls")
	iteratorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))

	iteratorDurationHistogram, _ := s.config.Meter.Float64Histogram("host_api_iterator_duration_ms")
	iteratorDurationHistogram.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs[:2]...))

	if err != nil {
		span.RecordError(err)
		return nil, false, err
	}

	// Auto-cleanup if no more data
	if !hasMore {
		info.iterator.Close()
		s.mu.Lock()
		delete(s.iterators, iteratorID)
		s.mu.Unlock()
	}

	return data, hasMore, nil
}

// CloseIterator cleans up iterator resources
func (s *defaultHostAPISet) CloseIterator(ctx context.Context, iteratorID string) error {
	s.mu.Lock()
	info, ok := s.iterators[iteratorID]
	if !ok {
		s.mu.Unlock()
		return nil // Already closed or never existed
	}

	delete(s.iterators, iteratorID)
	s.mu.Unlock()

	// Close iterator outside of lock to avoid potential deadlock
	err := info.iterator.Close()

	if s.config.Logger != nil {
		s.config.Logger.Debug("iterator closed",
			"iterator_id", iteratorID,
			"api", info.apiName,
			"method", info.method,
			"duration", time.Since(info.createdAt),
		)
	}

	return err
}

// CleanupStaleIterators removes iterators older than the timeout
func (s *defaultHostAPISet) CleanupStaleIterators() int {
	timeout := s.config.IteratorTimeout
	if timeout == 0 {
		timeout = DefaultIteratorTimeout
	}

	now := time.Now()

	s.mu.Lock()
	staleIterators := make(map[string]*iteratorInfo)
	for id, info := range s.iterators {
		if now.Sub(info.createdAt) > timeout {
			staleIterators[id] = info
			delete(s.iterators, id)
		}
	}
	s.mu.Unlock()

	// Close stale iterators outside of lock
	for id, info := range staleIterators {
		if err := info.iterator.Close(); err != nil {
			if s.config.Logger != nil {
				s.config.Logger.Error("failed to close stale iterator",
					"iterator_id", id,
					"api", info.apiName,
					"method", info.method,
					"age", now.Sub(info.createdAt),
					"error", err,
				)
			}
		}
	}

	return len(staleIterators)
}

// Close cleans up all host API resources
func (s *defaultHostAPISet) Close() error {
	// Defensive: prevent double close with proper locking
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil // Idempotent - already closed
	}

	// Mark as closed first to prevent new operations
	s.closed = true

	// Copy iterators to close outside of lock
	iteratorsToClose := make(map[string]*iteratorInfo)
	for id, info := range s.iterators {
		iteratorsToClose[id] = info
	}
	s.iterators = nil
	s.mu.Unlock()

	var errs []error

	// Close all active iterators
	for id, info := range iteratorsToClose {
		if err := info.iterator.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close iterator %s: %w", id, err))
		}
	}

	// Close each host API if it implements io.Closer
	for name, api := range s.apis {
		if closer, ok := api.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close %s: %w", name, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing host APIs: %v", errs)
	}

	return nil
}

// Config returns the configuration for this host API set
func (s *defaultHostAPISet) Config() HostAPIConfig {
	return s.config
}

// Context keys for passing data through the call stack
type (
	hostAPISetKey  struct{}
	serviceInfoKey struct{}
	memoryKey      struct{}
	moduleKey      struct{}
	iteratorKey    struct{ id string }
)

// Helper function to generate iterator IDs
func generateIteratorID() string {
	return uuid.New().String()
}
