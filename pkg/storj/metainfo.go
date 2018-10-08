// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"context"
)

// Metainfo represents a database for storing meta-info about objects
type Metainfo interface {
	// MetainfoLimits returns limits for this metainfo database
	Limits() (MetainfoLimits, error)

	// CreateBucket creates a new bucket with the specified information
	CreateBucket(ctx context.Context, bucket Bucket) (Bucket, error)
	// DeleteBucket deletes bucket
	DeleteBucket(ctx context.Context, bucket string) error
	// GetBucket gets bucket information
	GetBucket(ctx context.Context, bucket string) (Bucket, error)

	// GetObject returns information about an object
	GetObject(ctx context.Context, bucket string, path Path) (Object, error)
	// GetObjectStream returns interface for reading the object stream
	GetObjectStream(ctx context.Context, bucket string, path Path) (ReadonlyStream, error)

	// CreateObject creates an uploading object and returns an interface for uploading Object information
	CreateObject(ctx context.Context, bucket string, path Path, info Object) (MutableObject, error)
	// ModifyObject creates an interface for modifying an existing object
	ModifyObject(ctx context.Context, bucket string, path Path, info Object) (MutableObject, error)
	// DeleteObject deletes an object from database
	DeleteObject(ctx context.Context, bucket string, path Path) error

	// TODO: add things for continuing existing objects
	ListObjects(cctx context.Context, bucket string, options ListOptions) (ObjectList, error)
}

// ListOptions lists objects
type ListOptions struct {
	Prefix    Path
	First     Path // First is relative to Prefix, full path is Prefix + First
	Delimiter rune
	Recursive bool
	Limit     int

	// Token used for the next listing.
	Token string
}

// ObjectList is a list of objects
type ObjectList struct {
	Bucket string
	Prefix Path
	Token  string
	More   bool

	// Items paths are relative to Prefix
	// To get the full path use list.Prefix + list.Items[0].Path
	Items []Object
}

// MetainfoLimits lists limits specified for the Metainfo database
type MetainfoLimits struct {
	// ListLimit specifies the maximum amount of items that can be listed at a time.
	ListLimit int64

	// MaximumInlineSegment specifies the maximum inline segment that is allowed to be stored.
	MaximumInlineSegmentSize int64
}

// ReadonlyStream is an interface for reading segment information
type ReadonlyStream interface {
	Info() Object

	// SegmentsAt returns the segment that contains the byteOffset and following segments.
	// Limit specifies how much to return at most.
	// Returns io.EOF, when there aren't more segments.
	SegmentsAt(ctx context.Context, byteOffset int64, limit int64) ([]Segment, error)
	// Segments returns the segment at index.
	// Limit specifies how much to return at most.
	// Returns io.EOF, when there aren't more segments.
	Segments(ctx context.Context, index int64, limit int64) ([]Segment, error)
}

// MutableObject is an interface for manipulating creating/deleting object stream
type MutableObject interface {
	// Info gets the current information about the object
	Info() (Object, error)

	// CreateStream creates a new stream for the object
	CreateStream() (MutableStream, error)
	// ContinueStream starts to continue a partially uploaded stream.
	// ContinueStream() (MutableStream, error)
	// DeleteStream deletes any information about this objects stream
	DeleteStream() error

	// Commit commits the changes to the database
	Commit() error
}

// MutableStream is an interface for manipulating stream information
type MutableStream interface {
	ReadonlyStream

	// AddSegments adds segments to the stream.
	AddSegments(segments ...Segment) error
	// UpdateSegments updates information about segments.
	UpdateSegments(segments ...Segment) error
}
