// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"time"
)

// PendingResult represents an in-flight operation whose completion can be
// confirmed asynchronously. It is used to decouple submission from confirmation,
// enabling batched publishing without blocking the record processing loop.
type PendingResult interface {
	// Timestamp returns the time associated with this result. Used by the
	// drain loop to advance the watermark after confirmation.
	Timestamp() time.Time

	// Ready returns a channel that is closed when the result is ready.
	// When the Ready channel is closed, Get is guaranteed not to block.
	Ready() <-chan struct{}

	// Get blocks until the operation is confirmed or permanently failed.
	// Permanent errors (e.g. user misconfiguration) are handled internally
	// and result in a nil return. A non-nil error indicates an infrastructure
	// failure (e.g. context cancellation) that should abort processing.
	Get(ctx context.Context) error
}

// ImmediateResult returns a PendingResult that is already resolved with the
// given timestamp. Useful in callbacks that do not perform any async work.
func ImmediateResult(timestamp time.Time) PendingResult {
	return &immediateResult{timestamp: timestamp}
}

var closedChan = func() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()

type immediateResult struct{ timestamp time.Time }

func (r *immediateResult) Timestamp() time.Time        { return r.timestamp }
func (r *immediateResult) Ready() <-chan struct{}      { return closedChan }
func (r *immediateResult) Get(_ context.Context) error { return nil }

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
