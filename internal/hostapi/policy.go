package hostapi

import "context"

// PolicyEngine evaluates CEL-based policies
type PolicyEngine interface {
	// Evaluate checks if a host API call is allowed
	Evaluate(ctx context.Context, check PolicyCheck) (PolicyDecision, error)
}

// PolicyCheck represents a request to check a policy
type PolicyCheck struct {
	Service string                 // calling service (e.g., "acme-corp/user-service")
	Request HostAPIRequest         // the full request being evaluated
	Context map[string]interface{} // additional context for CEL (e.g., time, IP, etc.)
}

// PolicyDecision represents the result of a policy check
type PolicyDecision struct {
	Allowed  bool
	Reason   string
	Metadata map[string]interface{} // e.g., rate limit remaining
}
