// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

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
	EncryptedETag                 []byte

	Encryption storj.EncryptionParameters
}

// IteratePendingObjectsByKey contains arguments necessary for listing pending objects by ObjectKey.
type IteratePendingObjectsByKey struct {
	ObjectLocation
	BatchSize int
	Cursor    StreamIDCursor
}

// PendingObjectsIterator iterates over a sequence of PendingObjectEntry items.
type PendingObjectsIterator interface {
	Next(ctx context.Context, item *PendingObjectEntry) bool
}

// IteratePendingObjectsByKey iterates through all streams of pending objects with the same ObjectKey.
func (db *DB) IteratePendingObjectsByKey(ctx context.Context, opts IteratePendingObjectsByKey, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}
	return iteratePendingObjectsByKey(ctx, db.ChooseAdapter(opts.ProjectID), opts, fn)
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
