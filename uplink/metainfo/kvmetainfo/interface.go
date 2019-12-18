// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"

	"storj.io/storj/pkg/storj"
)

// ReadOnlyStream is an interface for reading segment information
type ReadOnlyStream interface {
	Info() storj.Object

	// SegmentsAt returns the segment that contains the byteOffset and following segments.
	// Limit specifies how much to return at most.
	SegmentsAt(ctx context.Context, byteOffset int64, limit int64) (infos []storj.Segment, more bool, err error)
	// Segments returns the segment at index.
	// Limit specifies how much to return at most.
	Segments(ctx context.Context, index int64, limit int64) (infos []storj.Segment, more bool, err error)
}

// MutableObject is an interface for manipulating creating/deleting object stream
type MutableObject interface {
	// Info gets the current information about the object
	Info() storj.Object

	// CreateStream creates a new stream for the object
	CreateStream(ctx context.Context) (MutableStream, error)
	// ContinueStream starts to continue a partially uploaded stream.
	ContinueStream(ctx context.Context) (MutableStream, error)
	// DeleteStream deletes any information about this objects stream
	DeleteStream(ctx context.Context) error

	// Commit commits the changes to the database
	Commit(ctx context.Context) error
}

// MutableStream is an interface for manipulating stream information
type MutableStream interface {
	// TODO: methods for finding partially uploaded segments

	Info() storj.Object
	// AddSegments adds segments to the stream.
	AddSegments(ctx context.Context, segments ...storj.Segment) error
	// UpdateSegments updates information about segments.
	UpdateSegments(ctx context.Context, segments ...storj.Segment) error
}
