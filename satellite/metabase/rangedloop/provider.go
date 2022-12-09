// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"

	"storj.io/storj/satellite/metabase/segmentloop"
)

// RangeSplitter splits a source of segments into ranges,
// so that multiple segments can be processed concurrently.
// It usually abstracts over a database.
// It is a subcomponent of the ranged segment loop.
type RangeSplitter interface {
	CreateRanges(nRanges int, batchSize int) ([]SegmentProvider, error)
}

// SegmentProvider iterates through a range of segments.
type SegmentProvider interface {
	Iterate(ctx context.Context, fn func([]segmentloop.Segment) error) error
}
