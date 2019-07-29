// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/metainfo"
	"storj.io/storj/uplink/storage/buckets"
	"storj.io/storj/uplink/storage/streams"
)

// Project implements project management operations
type Project struct {
	buckets            buckets.Store
	streams            streams.Store
	encryptedBlockSize int32
	redundancy         eestream.RedundancyStrategy
	segmentsSize       int64
}

// NewProject constructs a *Project
func NewProject(streams streams.Store, encryptedBlockSize int32, redundancy eestream.RedundancyStrategy, segmentsSize int64, metainfoClient metainfo.Client) *Project {
	return &Project{
		buckets:            buckets.NewStore(metainfoClient),
		streams:            streams,
		encryptedBlockSize: encryptedBlockSize,
		redundancy:         redundancy,
		segmentsSize:       segmentsSize,
	}
}
