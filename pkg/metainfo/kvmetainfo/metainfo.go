// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

var mon = monkit.Package()

var errClass = errs.Class("kvmetainfo")

const defaultSegmentLimit = 8 // TODO

var _ storj.Metainfo = (*DB)(nil)

// DB implements metainfo database
type DB struct {
	buckets  buckets.Store
	streams  streams.Store
	segments segments.Store
	pointers pdbclient.Client

	rootKey *storj.Key
}

// New creates a new metainfo database
func New(buckets buckets.Store, streams streams.Store, segments segments.Store, pointers pdbclient.Client, rootKey *storj.Key) *DB {
	return &DB{
		buckets:  buckets,
		streams:  streams,
		segments: segments,
		pointers: pointers,
		rootKey:  rootKey,
	}
}

// Limits returns limits for this metainfo database
func (db *DB) Limits() (storj.MetainfoLimits, error) {
	return storj.MetainfoLimits{
		ListLimit:                storage.LookupLimit,
		MinimumRemoteSegmentSize: int64(memory.KB), // TODO: is this needed here?
		MaximumInlineSegmentSize: int64(memory.MB),
	}, nil
}
