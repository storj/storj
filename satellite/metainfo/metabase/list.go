// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"storj.io/common/uuid"
)

// ObjectEntry contains information about an item in a bucket.
type ObjectEntry Object

// ObjectsIterator iterates over a sequence of ObjectEntry items.
type ObjectsIterator interface {
	Next(ctx context.Context, item *ObjectEntry) bool
}

// IterateCursor is a cursor used during iteration.
type IterateCursor struct {
	Key     ObjectKey
	Version Version
}

// IterateObjects contains arguments necessary for listing objects in a bucket.
type IterateObjects struct {
	ProjectID  uuid.UUID
	BucketName string
	Recursive  bool
	BatchSize  int
	Prefix     ObjectKey
	Cursor     IterateCursor
	Status     ObjectStatus
}

// IterateObjectsAllVersions iterates through all versions of all committed objects.
func (db *DB) IterateObjectsAllVersions(ctx context.Context, opts IterateObjects, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	if err = opts.Verify(); err != nil {
		return err
	}
	return iterateAllVersions(ctx, db, opts, fn)
}

// Verify verifies get object request fields.
func (opts *IterateObjects) Verify() error {
	switch {
	case opts.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case opts.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case !opts.Recursive:
		return ErrInvalidRequest.New("non-recursive listing not implemented yet")
	case opts.BatchSize < 0:
		return ErrInvalidRequest.New("BatchSize is negative")
	case !(opts.Status == Pending || opts.Status == Committed):
		return ErrInvalidRequest.New("Status %v is not supported", opts.Status)
	}
	return nil
}
