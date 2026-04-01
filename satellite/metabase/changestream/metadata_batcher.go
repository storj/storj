// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MetadataBatcher buffers watermark updates, state transitions, and child partition
// inserts in memory and flushes them to Spanner in a single transaction.
// This eliminates per-record Spanner round-trips from the processPartition hot path.
//
// All buffer methods (UpdateWatermark, UpdateState, AddChildPartition) are safe
// to call concurrently from multiple partition goroutines.
// Flush must be called from a single goroutine (the processLoop main goroutine).
type MetadataBatcher struct {
	adapter  Adapter
	feedName string
	log      *zap.Logger

	mu            sync.Mutex
	watermarks    map[string]time.Time
	states        map[string]PartitionState
	newPartitions []NewPartition
}

// NewMetadataBatcher creates a new MetadataBatcher.
func NewMetadataBatcher(log *zap.Logger, adapter Adapter, feedName string) *MetadataBatcher {
	return &MetadataBatcher{
		adapter:    adapter,
		feedName:   feedName,
		log:        log,
		watermarks: make(map[string]time.Time),
		states:     make(map[string]PartitionState),
	}
}

// UpdatePartitionWatermark buffers a watermark update for the given partition token.
// Last write wins — only the most recent watermark per partition is flushed.
func (b *MetadataBatcher) UpdatePartitionWatermark(partitionToken string, t time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.watermarks[partitionToken] = t
}

// UpdatePartitionState buffers state transitions for one or more partition tokens.
func (b *MetadataBatcher) UpdatePartitionState(state PartitionState, partitionTokens ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, partitionToken := range partitionTokens {
		b.states[partitionToken] = state
	}
}

// AddChildPartition buffers a child partition insert.
func (b *MetadataBatcher) AddChildPartition(childToken string, parentTokens []string, start time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.newPartitions = append(b.newPartitions, NewPartition{
		Token:        childToken,
		ParentTokens: parentTokens,
		Start:        start,
	})
}

// Flush writes all buffered updates to Spanner in a single transaction.
// Returns nil if there is nothing to flush.
func (b *MetadataBatcher) Flush(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	b.mu.Lock()
	updates := PartitionUpdates{
		Watermarks:    b.watermarks,
		States:        b.states,
		NewPartitions: b.newPartitions,
	}
	b.watermarks = make(map[string]time.Time)
	b.states = make(map[string]PartitionState)
	b.newPartitions = nil
	b.mu.Unlock()

	total := len(updates.Watermarks) + len(updates.States) + len(updates.NewPartitions)
	if total == 0 {
		return nil
	}

	b.log.Debug("Flushing partition metadata",
		zap.String("change_stream", b.feedName),
		zap.Int("watermarks", len(updates.Watermarks)),
		zap.Int("states", len(updates.States)),
		zap.Int("new_partitions", len(updates.NewPartitions)))

	err = b.adapter.UpdateChangeStreamPartitions(ctx, b.feedName, updates)
	if err != nil {
		return err
	}

	b.log.Debug("Flushed partition metadata",
		zap.String("change_stream", b.feedName),
		zap.Int("total", total))

	return nil
}
