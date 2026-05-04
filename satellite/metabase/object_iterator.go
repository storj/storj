// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"storj.io/common/uuid"
)

// ObjectIteratorOptions selects which objects an ObjectIterator yields
// and in what order.
type ObjectIteratorOptions struct {
	ProjectID   uuid.UUID
	BucketName  BucketName
	Prefix      ObjectKey
	PrefixLimit ObjectKey

	// Cursor is the (key, version, stream_id) position to resume from.
	Cursor    ObjectsIteratorCursor
	BatchSize int

	// Delimiter and Recursive together drive the cursor-skip-past-folder
	// optimization in non-recursive listings: between SQL batches, the
	// iterator advances the cursor past the current folder rather than
	// fetching and discarding every row in it.
	Delimiter ObjectKey
	Recursive bool

	// Mode selects which subset of objects is returned and the order.
	Mode ObjectIteratorMode

	// PendingOnly reverses the status filter: when true, only pending
	// objects are returned; otherwise, only non-pending objects are.
	PendingOnly bool

	IncludeCustomMetadata       bool
	IncludeSystemMetadata       bool
	IncludeETag                 bool
	IncludeETagOrCustomMetadata bool
	IncludeChecksum             bool
}

// ObjectIteratorMode selects the ordering and version-handling of an
// ObjectIterator.
type ObjectIteratorMode int

const (
	// ObjectIteratorModeAllVersionsDescending yields all matching object
	// versions ordered (key ASC, version DESC).
	ObjectIteratorModeAllVersionsDescending ObjectIteratorMode = iota
	// ObjectIteratorModeAllVersionsAscending yields all matching object
	// versions ordered (key ASC, version ASC).
	ObjectIteratorModeAllVersionsAscending
	// ObjectIteratorModePendingByKey yields pending objects under a single
	// (project, bucket, key), ordered by (version, stream_id).
	ObjectIteratorModePendingByKey
)

// ObjectIterator is a cursor over object rows returned by a backend adapter.
type ObjectIterator interface {
	// Next advances to the next row and copies it into dst. It returns
	// (true, nil) when dst was populated, (false, nil) at end of
	// iteration, and (false, err) on failure. The set of fields
	// populated depends on the IncludeXxx options passed when the
	// iterator was opened.
	Next(ctx context.Context, dst *ObjectEntry) (bool, error)
	// Close releases iterator resources. Safe to call multiple times.
	Close() error
}
