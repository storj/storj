// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"strings"
	"time"

	"github.com/zeebo/errs"

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
	return NewStreamVersionID(entry.Version, entry.StreamID)
}

// Less implements sorting on object entries.
func (entry ObjectEntry) Less(other ObjectEntry) bool {
	return ObjectStream{
		ObjectKey: entry.ObjectKey,
		Version:   entry.Version,
		StreamID:  entry.StreamID,
	}.Less(ObjectStream{
		ObjectKey: other.ObjectKey,
		Version:   other.Version,
		StreamID:  other.StreamID,
	})
}

// LessVersionAsc implements sorting on object entries.
func (entry ObjectEntry) LessVersionAsc(other ObjectEntry) bool {
	return ObjectStream{
		ObjectKey: entry.ObjectKey,
		Version:   entry.Version,
		StreamID:  entry.StreamID,
	}.LessVersionAsc(ObjectStream{
		ObjectKey: other.ObjectKey,
		Version:   other.Version,
		StreamID:  other.StreamID,
	})
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

// IterateObjectsWithStatus contains arguments necessary for listing objects in a bucket.
type IterateObjectsWithStatus struct {
	ProjectID             uuid.UUID
	BucketName            BucketName
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
	return iterateAllVersionsWithStatusDescending(ctx, db.ChooseAdapter(opts.ProjectID), opts, fn)
}

// IterateObjectsAllVersionsWithStatusAscending iterates through all versions of all objects with specified status. Ordered from oldest to latest.
// TODO this method was copied (and renamed) from v1.95.1 as a workaround for issues with metabase.ListObject performance. It should be removed
// when problem with metabase.ListObject will be fixed.
func (db *DB) IterateObjectsAllVersionsWithStatusAscending(ctx context.Context, opts IterateObjectsWithStatus, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	if err = opts.Verify(); err != nil {
		return err
	}
	return iterateAllVersionsWithStatusAscending(ctx, db.ChooseAdapter(opts.ProjectID), opts, fn)
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

// ListObjectsWithIterator lists objects.
func (db *DB) ListObjectsWithIterator(ctx context.Context, opts ListObjects) (result ListObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return ListObjectsResult{}, err
	}
	if opts.Pending || opts.AllVersions {
		return ListObjectsResult{}, errs.New("not implemented")
	}

	ListLimit.Ensure(&opts.Limit)

	err = db.IterateObjectsAllVersionsWithStatus(ctx,
		IterateObjectsWithStatus{
			ProjectID:  opts.ProjectID,
			BucketName: opts.BucketName,
			Prefix:     opts.Prefix,
			Cursor: IterateCursor{
				Key:     opts.Cursor.Key,
				Version: MaxVersion,
			},
			Recursive: opts.Recursive,
			// TODO we may need to increase batch size to optimize number
			// of DB calls for objects with multiple versions
			BatchSize:             opts.Limit + 1,
			Pending:               false,
			IncludeCustomMetadata: opts.IncludeCustomMetadata,
			IncludeSystemMetadata: opts.IncludeSystemMetadata,
		}, func(ctx context.Context, it ObjectsIterator) error {
			var previousLatestSet bool
			var entry, previousLatest ObjectEntry
			prefix := opts.Prefix
			if prefix != "" && !strings.HasSuffix(string(prefix), "/") {
				prefix += "/"
			}

			for len(result.Objects) < opts.Limit && it.Next(ctx, &entry) {
				objectKey := prefix + entry.ObjectKey
				if opts.Cursor.Key == objectKey && opts.Cursor.Version >= entry.Version {
					previousLatestSet = true
					previousLatest = entry
					continue
				}

				if entry.Status.IsDeleteMarker() && (!previousLatestSet || prefix+previousLatest.ObjectKey != objectKey) {
					previousLatestSet = true
					previousLatest = entry
					continue
				}

				if !previousLatestSet || prefix+previousLatest.ObjectKey != objectKey {
					previousLatestSet = true
					previousLatest = entry

					result.Objects = append(result.Objects, entry)
				}
			}

			result.More = it.Next(ctx, &entry)
			return nil
		},
	)
	if err != nil {
		return ListObjectsResult{}, err
	}
	return result, nil
}
