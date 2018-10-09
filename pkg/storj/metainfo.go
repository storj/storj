// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"context"
	"time"
)

// Metainfo represents a database for storing meta-info about objects
type Metainfo interface {
	// MetainfoLimits returns limits for this metainfo database
	Limits() (MetainfoLimits, error)

	// CreateBucket creates a new bucket with the specified information
	// Database automatically sets different values in the information
	CreateBucket(ctx context.Context, bucket string, info *Bucket) (Bucket, error)
	// DeleteBucket deletes bucket
	DeleteBucket(ctx context.Context, bucket string) error
	// GetBucket gets bucket information
	GetBucket(ctx context.Context, bucket string) (Bucket, error)

	// GetObject returns information about an object
	GetObject(ctx context.Context, bucket string, path Path) (Object, error)
	// GetObjectStream returns interface for reading the object stream
	GetObjectStream(ctx context.Context, bucket string, path Path) (ReadOnlyStream, error)

	// CreateObject creates an uploading object and returns an interface for uploading Object information
	CreateObject(ctx context.Context, bucket string, path Path, info *CreateObject) (MutableObject, error)
	// ModifyObject creates an interface for modifying an existing object
	ModifyObject(ctx context.Context, bucket string, path Path, info Object) (MutableObject, error)
	// DeleteObject deletes an object from database
	DeleteObject(ctx context.Context, bucket string, path Path) error

	// ListObjects lists objects in bucket based on the ListOptions
	ListObjects(ctx context.Context, bucket string, options ListOptions) (ObjectList, error)
}

// CreateObject has optional parameters that can be set
type CreateObject struct {
	Metadata    []byte
	ContentType string
	Expires     time.Time
}

// Object converts the CreateObject to an object with unitialized values
func (create CreateObject) Object(bucket string, path Path) Object {
	return Object{
		Bucket:      bucket,
		Path:        path,
		Metadata:    create.Metadata,
		ContentType: create.ContentType,
		Expires:     create.Expires,
	}
}

// ListOptions lists objects
type ListOptions struct {
	Prefix    Path
	First     Path // First is relative to Prefix, full path is Prefix + First
	Delimiter rune
	Recursive bool
	Limit     int
}

// ObjectList is a list of objects
type ObjectList struct {
	Bucket string
	Prefix Path

	NextFirst Path // relative to Prefix, to get the full path use Prefix + NextFirst
	More      bool

	// Items paths are relative to Prefix
	// To get the full path use list.Prefix + list.Items[0].Path
	Items []Object
}

// MetainfoLimits lists limits specified for the Metainfo database
type MetainfoLimits struct {
	// ListLimit specifies the maximum amount of items that can be listed at a time.
	ListLimit int64

	// MinimumRemoteSegmentSize specifies the minimum remote segment that is allowed to be stored.
	MinimumRemoteSegmentSize int64
	// MaximumInlineSegmentSize specifies the maximum inline segment that is allowed to be stored.
	MaximumInlineSegmentSize int64
}

// ReadOnlyStream is an interface for reading segment information
type ReadOnlyStream interface {
	Info() Object

	// SegmentsAt returns the segment that contains the byteOffset and following segments.
	// Limit specifies how much to return at most.
	SegmentsAt(ctx context.Context, byteOffset int64, limit int64) (infos []Segment, more bool, err error)
	// Segments returns the segment at index.
	// Limit specifies how much to return at most.
	Segments(ctx context.Context, index int64, limit int64) (infos []Segment, more bool, err error)
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
	ReadOnlyStream

	// AddSegments adds segments to the stream.
	AddSegments(segments ...Segment) error
	// UpdateSegments updates information about segments.
	UpdateSegments(segments ...Segment) error
}
