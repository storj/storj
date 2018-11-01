// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

var errClass = errs.Class("kvmetainfo")

const defaultSegmentLimit = 8 // TODO

var _ storj.Metainfo = (*DB)(nil)

// DB implements metainfo database
type DB struct {
	*Buckets
	*Objects
}

// TODO: ensure this only needs pointerdb for implementation

// New creates a new metainfo database
func New(buckets buckets.Store, objects objects.Store, streams streams.Store, segments segments.Store) *DB {
	return &DB{
		Buckets: NewBuckets(buckets),
		Objects: NewObjects(objects, streams, segments),
	}
}

func (db *DB) Limits() (storj.MetainfoLimits, error) {
	return storj.MetainfoLimits{
		ListLimit:                storage.LookupLimit,
		MinimumRemoteSegmentSize: memory.KB, // TODO: is this needed here?
		MaximumInlineSegmentSize: memory.MB,
	}, nil
}
