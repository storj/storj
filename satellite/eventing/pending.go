// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"time"

	"storj.io/storj/satellite/metabase/changestream"
)

// PendingResult is an alias for changestream.PendingResult.
// It is defined here so that future backends (e.g. TiDBEventSource) can
// reference the type without importing the Spanner-specific changestream package.
type PendingResult = changestream.PendingResult

// ImmediateResult returns a PendingResult that is already resolved with the
// given timestamp. Delegates to changestream.ImmediateResult.
func ImmediateResult(timestamp time.Time) PendingResult {
	return changestream.ImmediateResult(timestamp)
}

// CombinedPendingResult is a PendingResult that resolves only after all
// underlying results resolve. Used when a single source record produces
// multiple ChangeEvents (e.g. delete-all-bucket-objects).
type CombinedPendingResult struct {
	results []PendingResult
}

// NewCombinedPendingResult creates a CombinedPendingResult from a slice of results.
// Panics if results is empty.
func NewCombinedPendingResult(results []PendingResult) *CombinedPendingResult {
	if len(results) == 0 {
		panic("NewCombinedPendingResult: results must not be empty")
	}
	return &CombinedPendingResult{results: results}
}

// Timestamp returns the timestamp of the last result.
func (c *CombinedPendingResult) Timestamp() time.Time {
	return c.results[len(c.results)-1].Timestamp()
}

// Ready returns a channel that is closed when all underlying results are ready.
func (c *CombinedPendingResult) Ready() <-chan struct{} {
	if len(c.results) == 1 {
		return c.results[0].Ready()
	}
	merged := make(chan struct{})
	go func() {
		for _, r := range c.results {
			<-r.Ready()
		}
		close(merged)
	}()
	return merged
}

// Get blocks until all underlying results are confirmed or one permanently fails.
func (c *CombinedPendingResult) Get(ctx context.Context) error {
	for _, r := range c.results {
		if err := r.Get(ctx); err != nil {
			return err
		}
	}
	return nil
}
