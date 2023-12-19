// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// ObjectEntry contains information about an item in a bucket.
type ObjectEntry struct {
	IsPrefix bool

	ObjectKey ObjectKey
	Version   Version
	StreamID  uuid.UUID

	CreatedAt time.Time
	ExpiresAt *time.Time

	Status       ObjectStatus
	SegmentCount int32

	EncryptedMetadataNonce        []byte
	EncryptedMetadata             []byte
	EncryptedMetadataEncryptedKey []byte

	TotalPlainSize     int64
	TotalEncryptedSize int64
	FixedSegmentSize   int32

	Encryption storj.EncryptionParameters
}

// StreamVersionID returns byte representation of object stream version id.
func (entry ObjectEntry) StreamVersionID() StreamVersionID {
	return newStreamVersionID(entry.Version, entry.StreamID)
}

// ObjectsIterator iterates over a sequence of ObjectEntry items.
type ObjectsIterator interface {
	Next(ctx context.Context, item *ObjectEntry) bool
}

// IterateCursor is a cursor used during iteration through objects.
//
// The cursor is exclusive.
type IterateCursor struct {
	Key     ObjectKey
	Version Version
}

// StreamIDCursor is a cursor used during iteration through streamIDs of a pending object.
type StreamIDCursor struct {
	StreamID uuid.UUID
}

// IteratePendingObjectsByKey contains arguments necessary for listing pending objects by ObjectKey.
type IteratePendingObjectsByKey struct {
	ObjectLocation
	BatchSize int
	Cursor    StreamIDCursor
}

// IterateObjectsWithStatus contains arguments necessary for listing objects in a bucket.
type IterateObjectsWithStatus struct {
	ProjectID             uuid.UUID
	BucketName            string
	Recursive             bool
	BatchSize             int
	Prefix                ObjectKey
	Cursor                IterateCursor
	Pending               bool
	IncludeCustomMetadata bool
	IncludeSystemMetadata bool
}

// IterateObjectsAllVersionsWithStatus iterates through all versions of all objects with specified status.
func (db *DB) IterateObjectsAllVersionsWithStatus(ctx context.Context, opts IterateObjectsWithStatus, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	if err = opts.Verify(); err != nil {
		return err
	}
	return iterateAllVersionsWithStatus(ctx, db, opts, fn)
}

// Verify verifies get object request fields.
func (opts *IterateObjectsWithStatus) Verify() error {
	switch {
	case opts.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case opts.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case opts.BatchSize < 0:
		return ErrInvalidRequest.New("BatchSize is negative")
	}
	return nil
}

// IteratePendingObjectsByKey iterates through all streams of pending objects with the same ObjectKey.
func (db *DB) IteratePendingObjectsByKey(ctx context.Context, opts IteratePendingObjectsByKey, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}
	return iteratePendingObjectsByKey(ctx, db, opts, fn)
}

// Verify verifies get object request fields.
func (opts *IteratePendingObjectsByKey) Verify() error {
	if err := opts.ObjectLocation.Verify(); err != nil {
		return err
	}
	if opts.BatchSize < 0 {
		return ErrInvalidRequest.New("BatchSize is negative")
	}
	return nil
}

// PendingObjectEntry contains information about an pending object item in a bucket.
type PendingObjectEntry struct {
	IsPrefix bool

	ObjectKey ObjectKey
	StreamID  uuid.UUID

	CreatedAt time.Time
	ExpiresAt *time.Time

	EncryptedMetadataNonce        []byte
	EncryptedMetadata             []byte
	EncryptedMetadataEncryptedKey []byte

	Encryption storj.EncryptionParameters
}

// PendingObjectsIterator iterates over a sequence of PendingObjectEntry items.
type PendingObjectsIterator interface {
	Next(ctx context.Context, item *PendingObjectEntry) bool
}

// PendingObjectsCursor cursor for iterating over pending objects.
type PendingObjectsCursor struct {
	Key      ObjectKey
	StreamID uuid.UUID
}

// IteratePendingObjects contains arguments necessary for listing pending objects in a bucket.
type IteratePendingObjects struct {
	ProjectID             uuid.UUID
	BucketName            string
	Recursive             bool
	BatchSize             int
	Prefix                ObjectKey
	Cursor                PendingObjectsCursor
	IncludeCustomMetadata bool
	IncludeSystemMetadata bool
}

// Verify verifies request fields.
func (opts *IteratePendingObjects) Verify() error {
	switch {
	case opts.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case opts.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case opts.BatchSize < 0:
		return ErrInvalidRequest.New("BatchSize is negative")
	}
	return nil
}
