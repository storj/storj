// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
)

// RangeSplitter splits a source of segments into ranges,
// so that multiple segments can be processed concurrently.
// It usually abstracts over a database.
// It is a subcomponent of the ranged segment loop.
type RangeSplitter interface {
	CreateRanges(ctx context.Context, nRanges int, batchSize int) ([]SegmentProvider, error)
}

// SegmentProvider iterates through a range of segments.
type SegmentProvider interface {
	Range() UUIDRange
	Iterate(ctx context.Context, fn func([]Segment) error) error
}
