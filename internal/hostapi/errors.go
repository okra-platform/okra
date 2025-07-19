package hostapi

import "time"

// Resource limit constants
const (
	// DefaultMaxIteratorsPerService is the default maximum number of concurrent iterators per service
	DefaultMaxIteratorsPerService = 100

	// DefaultIteratorTimeout is the default timeout for idle iterators
	DefaultIteratorTimeout = 5 * time.Minute

	// DefaultMaxRequestSize is the default maximum size for incoming requests (10MB)
	DefaultMaxRequestSize = 10 * 1024 * 1024

	// DefaultMaxResponseSize is the default maximum size for outgoing responses (10MB)
	DefaultMaxResponseSize = 10 * 1024 * 1024

	// ErrorCodeResponseTooLarge indicates the response exceeded size limits
	ErrorCodeResponseTooLarge = "RESPONSE_TOO_LARGE"

	// ErrorCodeHostAPISetClosed indicates operations on a closed HostAPISet
	ErrorCodeHostAPISetClosed = "HOST_API_SET_CLOSED"

	// ErrorCodeAPINotFound indicates the requested API doesn't exist
	ErrorCodeAPINotFound = "API_NOT_FOUND"

	// ErrorCodePolicyError indicates policy evaluation failed
	ErrorCodePolicyError = "POLICY_ERROR"

	// ErrorCodePolicyDenied indicates the request was denied by policy
	ErrorCodePolicyDenied = "POLICY_DENIED"

	// ErrorCodeInternalError indicates an unexpected error occurred
	ErrorCodeInternalError = "INTERNAL_ERROR"

	// ErrorCodeIteratorNotFound indicates the iterator ID is invalid
	ErrorCodeIteratorNotFound = "ITERATOR_NOT_FOUND"

	// ErrorCodeIteratorLimitExceeded indicates too many concurrent iterators
	ErrorCodeIteratorLimitExceeded = "ITERATOR_LIMIT_EXCEEDED"
)

// WASM memory error indicators
const (
	// NullPointer indicates a null pointer error in WASM memory operations
	NullPointer = uint32(0)

	// ZeroLength indicates zero length in WASM memory operations
	ZeroLength = uint32(0)

	// ErrorBitMask is used to indicate an error in the high bit of the return value
	ErrorBitMask = uint64(1 << 63)
)

// HostAPIError provides structured error information
type HostAPIError struct {
	Code    string `json:"code"`              // e.g., "PERMISSION_DENIED"
	Message string `json:"message"`           // Human-readable error message
	Details string `json:"details,omitempty"` // Additional error context
}

// Error implements the error interface
func (e *HostAPIError) Error() string {
	if e.Details != "" {
		return e.Code + ": " + e.Message + " - " + e.Details
	}
	return e.Code + ": " + e.Message
}
