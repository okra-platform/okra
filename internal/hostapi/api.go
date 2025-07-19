package hostapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/go-openapi/spec"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// HostAPI defines the interface that all host APIs must implement
type HostAPI interface {
	// Name returns the namespace for this API (e.g., "okra.state")
	Name() string

	// Version returns the semantic version of this API
	Version() string

	// Execute handles a method call with JSON parameters
	Execute(ctx context.Context, method string, parameters json.RawMessage) (json.RawMessage, error)
}

// StreamingHostAPI extends HostAPI to support iterator-based methods
type StreamingHostAPI interface {
	HostAPI

	// ExecuteStreaming handles streaming method calls
	// Returns the response JSON and optionally an iterator
	ExecuteStreaming(ctx context.Context, method string, parameters json.RawMessage) (json.RawMessage, Iterator, error)
}

// HostAPIFactory creates instances of a host API for specific services
type HostAPIFactory interface {
	// Name returns the namespace for this API (e.g., "okra.state")
	Name() string

	// Version returns the semantic version of this API
	Version() string

	// Create creates a new instance of the host API for a specific service
	Create(ctx context.Context, config HostAPIConfig) (HostAPI, error)

	// Methods returns metadata about available methods for stub generation
	Methods() []MethodMetadata
}

// MethodMetadata provides information about a host API method
type MethodMetadata struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  *spec.Schema    `json:"parameters"` // JSON schema for parameters
	Returns     *spec.Schema    `json:"returns"`    // JSON schema for return value
	Errors      []ErrorMetadata `json:"errors"`
	Streaming   bool            `json:"streaming"` // True if this method returns an iterator
}

// ErrorMetadata describes possible errors a method can return
type ErrorMetadata struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// HostAPIConfig provides configuration to a host API during initialization
type HostAPIConfig struct {
	// Runtime configuration
	ServiceName    string // Fully qualified service name (e.g., "acme-corp/user-service")
	ServiceVersion string // Full semantic version (e.g., "v1.2.3", not just "v1")
	Environment    string // Deployment environment (e.g., "production", "staging", "development")

	// Policy engine for CEL evaluation
	PolicyEngine PolicyEngine

	// Telemetry providers
	Tracer trace.Tracer
	Meter  metric.Meter
	Logger *slog.Logger

	// Service-specific configuration from okra.json
	Config interface{} // The full okra.json configuration

	// Resource limits
	MaxIteratorsPerService int           // Maximum concurrent iterators (0 = use DefaultMaxIteratorsPerService)
	IteratorTimeout        time.Duration // Iterator idle timeout (0 = use DefaultIteratorTimeout)
	MaxRequestSize         int           // Maximum request size in bytes (0 = use DefaultMaxRequestSize)
	MaxResponseSize        int           // Maximum response size in bytes (0 = use DefaultMaxResponseSize)
}

// HostAPIRequest represents a request to any host API
type HostAPIRequest struct {
	API        string          `json:"api"`        // e.g., "okra.state"
	Method     string          `json:"method"`     // e.g., "get"
	Parameters json.RawMessage `json:"parameters"` // Method-specific parameters
	Metadata   RequestMetadata `json:"metadata"`   // Request context, trace info, etc.
}

// HostAPIResponse represents the response from any host API
type HostAPIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`  // Success response data
	Error   *HostAPIError   `json:"error,omitempty"` // Error details if failed
}

// RequestMetadata carries request context
type RequestMetadata struct {
	TraceID     string            `json:"traceId,omitempty"`
	SpanID      string            `json:"spanId,omitempty"`
	Baggage     map[string]string `json:"baggage,omitempty"`
	ServiceInfo ServiceInfo       `json:"serviceInfo"`
}

// ServiceInfo identifies the calling service
type ServiceInfo struct {
	Name    string `json:"name"`    // e.g., "acme-corp/user-service"
	Version string `json:"version"` // e.g., "v1.2.3"
}

// NextRequest represents a request to get the next chunk from an iterator
type NextRequest struct {
	IteratorID string `json:"iteratorId"` // The iterator to advance
}

// NextResponse represents the response from a next() call
type NextResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`    // Next chunk of data
	HasMore bool            `json:"hasMore,omitempty"` // True if more data available
	Error   *HostAPIError   `json:"error,omitempty"`   // Error details if failed
}

// StreamingResponse is returned by streaming methods
type StreamingResponse struct {
	IteratorID string `json:"iteratorId"` // ID to use for subsequent next() calls
	HasData    bool   `json:"hasData"`    // False if no results at all
}
