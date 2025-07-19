package hostapi

import (
	"context"
	"encoding/json"
	"time"
)

// Iterator represents an active iterator for streaming data
// Note: We don't use Go 1.23's iter.Seq pattern here because:
// 1. We need explicit resource management (Close method)
// 2. Iterators must be serializable across WASM boundary
// 3. We need to track iterator state on the host side
// 4. Error handling needs to be explicit, not panic-based
type Iterator interface {
	// Next returns the next chunk of data and whether more data is available
	Next(ctx context.Context) (json.RawMessage, bool, error)
	// Close cleans up iterator resources
	Close() error
}

// iteratorInfo tracks iterator metadata
type iteratorInfo struct {
	iterator  Iterator
	apiName   string
	method    string
	createdAt time.Time
}
