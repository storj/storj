// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
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

/*
// New creates a new metainfo database
func New(client psclient.Client, rootKey *storj.Key) *DB {
	return &DB{
		// Buckets: NewBuckets(buckets),
		// Objects: NewObjects(objects, streams, segments),
	}
}
*/

func (db *DB) Limits() (storj.MetainfoLimits, error) {
	return storj.MetainfoLimits{
		ListLimit:                storage.LookupLimit,
		MinimumRemoteSegmentSize: int64(memory.KB), // TODO: is this needed here?
		MaximumInlineSegmentSize: int64(memory.MB),
	}, nil
}
