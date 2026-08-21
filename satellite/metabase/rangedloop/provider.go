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

	// ReadsSnapshot reports whether the scan reads one snapshot of the
	// segments: the source is immutable by construction, or every range is
	// read at one fixed timestamp that every backend serves. A backend that
	// cannot serve the timestamp (Postgres) fails the scan, unless live reads
	// were allowed for a metabase where no backend could have served one; see
	// CanReadSnapshot. Garbage collection refuses to generate bloom filters
	// from a scan that reads no snapshot, as it could miss the pieces of a
	// concurrent server-side copy.
	ReadsSnapshot() bool
}

// SegmentProvider iterates through a range of segments.
type SegmentProvider interface {
	Range() UUIDRange
	Iterate(ctx context.Context, fn func([]Segment) error) error
}
