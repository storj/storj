// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"storj.io/storj/satellite/metabase/changestream"
)

// ReadChangeStreamPartition reads records from a change stream partition and processes records via callback.
func (s *SpannerAdapter) ReadChangeStreamPartition(ctx context.Context, name string, partitionToken string, from time.Time, callback func(record changestream.ChangeRecord) error) error {
	return changestream.ReadPartition(ctx, s.log, s.client, name, partitionToken, from, callback)
}

// ChangeStreamNoPartitionMetadata checks if the metadata table for the change stream is empty.
func (s *SpannerAdapter) ChangeStreamNoPartitionMetadata(ctx context.Context, feedName string) (bool, error) {
	return changestream.NoPartitionMetadata(ctx, s.client, feedName)
}

// GetChangeStreamPartitionsByState retrieves change stream partitions by their state from the metabase.
func (s *SpannerAdapter) GetChangeStreamPartitionsByState(ctx context.Context, name string, state changestream.PartitionState) (map[string]time.Time, error) {
	return changestream.GetPartitionsByState(ctx, s.client, name, state)
}

// AddChangeStreamPartition adds a child partition to the metabase.
func (s *SpannerAdapter) AddChangeStreamPartition(ctx context.Context, feedName, childToken string, parentTokens []string, start time.Time) error {
	return changestream.AddChildPartition(ctx, s.client, feedName, childToken, parentTokens, start)
}

// ScheduleChangeStreamPartitions checks each partition in created state, and if all its parent partitions are finished, it will update its state to scheduled.
func (s *SpannerAdapter) ScheduleChangeStreamPartitions(ctx context.Context, feedName string) (int64, error) {
	return changestream.SchedulePartitions(ctx, s.client, feedName)
}

// UpdateChangeStreamPartitionWatermark updates the watermark for a change stream partition in the metabase.
func (s *SpannerAdapter) UpdateChangeStreamPartitionWatermark(ctx context.Context, feedName, partitionToken string, newWatermark time.Time) error {
	return changestream.UpdatePartitionWatermark(ctx, s.client, feedName, partitionToken, newWatermark)
}

// UpdateChangeStreamPartitionState updates the watermark for a change stream partition in the metabase.
func (s *SpannerAdapter) UpdateChangeStreamPartitionState(ctx context.Context, feedName, partitionToken string, newState changestream.PartitionState) error {
	return changestream.UpdatePartitionState(ctx, s.client, feedName, partitionToken, newState)
}

// TestCreateChangeStream creates a change stream for testing purposes.
func (s *SpannerAdapter) TestCreateChangeStream(ctx context.Context, name string) error {
	return changestream.TestCreateChangeStream(ctx, s.adminClient, s.connParams.DatabasePath(), name)
}

// TestDeleteChangeStream deletes the change stream with the given name for testing purposes.
func (s *SpannerAdapter) TestDeleteChangeStream(ctx context.Context, name string) error {
	return changestream.TestDeleteChangeStream(ctx, s.adminClient, s.connParams.DatabasePath(), name)
}

// TestCreateChangeStreamMetadata creates only the metadata table and index for testing purposes.
func (s *SpannerAdapter) TestCreateChangeStreamMetadata(ctx context.Context, name string) error {
	return changestream.TestCreateChangeStreamMetadata(ctx, s.adminClient, s.connParams.DatabasePath(), name)
}

// TestDeleteChangeStreamMetadata deletes only the metadata table and index for testing purposes.
func (s *SpannerAdapter) TestDeleteChangeStreamMetadata(ctx context.Context, name string) error {
	return changestream.TestDeleteChangeStreamMetadata(ctx, s.adminClient, s.connParams.DatabasePath(), name)
}
