// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"time"
)

// CreateObject has optional parameters that can be set
type CreateObject struct {
	Metadata    map[string]string
	ContentType string
	Expires     time.Time

	RedundancyScheme
	EncryptionParameters
}

// Object converts the CreateObject to an object with unitialized values
func (create CreateObject) Object(bucket Bucket, path Path) Object {
	return Object{
		Bucket:      bucket,
		Path:        path,
		Metadata:    create.Metadata,
		ContentType: create.ContentType,
		Expires:     create.Expires,
		Stream: Stream{
			Size:             -1,  // unknown
			Checksum:         nil, // unknown
			SegmentCount:     -1,  // unknown
			FixedSegmentSize: -1,  // unknown

			RedundancyScheme:     create.RedundancyScheme,
			EncryptionParameters: create.EncryptionParameters,
		},
	}
}

// ListDirection specifies listing direction
type ListDirection int8

const (
	// Before lists backwards from cursor, without cursor [NOT SUPPORTED]
	Before = ListDirection(-2)
	// Backward lists backwards from cursor, including cursor [NOT SUPPORTED]
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

	return ListOptions{
		Prefix:    opts.Prefix,
		Cursor:    list.Items[len(list.Items)-1].Path,
		Direction: After,
		Limit:     opts.Limit,
	}
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

	return BucketListOptions{
		Cursor:    list.Items[len(list.Items)-1].Name,
		Direction: After,
		Limit:     opts.Limit,
	}
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
