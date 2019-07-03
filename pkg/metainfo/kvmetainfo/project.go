// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
)

// Project implements project management operations
type Project struct {
	buckets            objects.Store
	streams            streams.Store
	encryptedBlockSize int32
	redundancy         eestream.RedundancyStrategy
	segmentsSize       int64
}

// NewProject constructs a *Project
func NewProject(streams streams.Store, encryptedBlockSize int32, redundancy eestream.RedundancyStrategy, segmentsSize int64) *Project {
	return &Project{
		buckets:            objects.NewStore(streams, storj.EncNull),
		streams:            streams,
		encryptedBlockSize: encryptedBlockSize,
		redundancy:         redundancy,
		segmentsSize:       segmentsSize,
	}
}
