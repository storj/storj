// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"storj.io/storj/satellite/metabase/changestream"
)

// ChangeStream listens to Spanner change stream and processes records via callback.
func (s *SpannerAdapter) ChangeStream(ctx context.Context, name string, partitionToken string, from time.Time, callback func(record changestream.DataChangeRecord) error) ([]changestream.ChildPartitionsRecord, error) {
	return changestream.ReadPartitions(ctx, s.log, s.client, name, partitionToken, from, callback)
}

// TestCreateChangeStream creates a change stream for testing purposes.
func (s *SpannerAdapter) TestCreateChangeStream(ctx context.Context, name string) error {
	return changestream.TestCreateChangeStream(ctx, s.adminClient, s.connParams.DatabasePath(), name)
}

// TestDeleteChangeStream deletes the change stream with the given name.
func (s *SpannerAdapter) TestDeleteChangeStream(ctx context.Context, name string) error {
	return changestream.TestDeleteChangeStream(ctx, s.adminClient, s.connParams.DatabasePath(), name)
}
