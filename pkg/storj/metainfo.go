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
	// ListBuckets lists buckets starting from first
	ListBuckets(ctx context.Context, options BucketListOptions) (BucketList, error)

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

// ListDirection specifies listing direction
type ListDirection int8

const (
	// Before lists backwards from cursor, without cursor
	Before = ListDirection(-2)
	// Backward lists backwards from cursor, including cursor
	Backward = ListDirection(-1)
	// Forward lists forwards from cursor, including cursor
	Forward = ListDirection(1)
	// After lists forwards from cursor, without cursor
	After = ListDirection(2)
)

// ListOptions lists objects
type ListOptions struct {
	Prefix    Path
	Cursor    Path // Cursor is relative to Prefix, full path is Prefix + Cursor
	Delimiter rune
	Recursive bool
	Direction ListDirection
	Limit     int
}

// ObjectList is a list of objects
type ObjectList struct {
	Bucket string
	Prefix Path
	More   bool

	// Items paths are relative to Prefix
	// To get the full path use list.Prefix + list.Items[0].Path
	Items []Object
}

// NextPage returns options for listing the next page
func (opts ListOptions) NextPage(list ObjectList) ListOptions {
	if !list.More || len(list.Items) == 0 {
		return ListOptions{}
	}

	switch opts.Direction {
	case Before, Backward:
		return ListOptions{
			Prefix:    opts.Prefix,
			Cursor:    list.Items[0].Path,
			Direction: Before,
			Limit:     opts.Limit,
		}
	case After, Forward:
		return ListOptions{
			Prefix:    opts.Prefix,
			Cursor:    list.Items[len(list.Items)-1].Path,
			Direction: After,
			Limit:     opts.Limit,
		}
	}

	return ListOptions{}
}

// BucketListOptions lists objects
type BucketListOptions struct {
	Cursor    string
	Direction ListDirection
	Limit     int
}

// BucketList is a list of buckets
type BucketList struct {
	More  bool
	Items []Bucket
}

// NextPage returns options for listing the next page
func (opts BucketListOptions) NextPage(list BucketList) BucketListOptions {
	if !list.More || len(list.Items) == 0 {
		return BucketListOptions{}
	}

	switch opts.Direction {
	case Before, Backward:
		return BucketListOptions{
			Cursor:    list.Items[0].Name,
			Direction: Before,
			Limit:     opts.Limit,
		}
	case After, Forward:
		return BucketListOptions{
			Cursor:    list.Items[len(list.Items)-1].Name,
			Direction: After,
			Limit:     opts.Limit,
		}
	}

	return BucketListOptions{}
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
