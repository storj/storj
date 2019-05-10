// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/storage/buckets"
)

// Project implements project management operations
type Project struct {
	buckets            buckets.Store
	encryptedBlockSize int32
	redundancy         eestream.RedundancyStrategy
	segmentsSize       int64
}

// NewProject constructs a *Project
func NewProject(buckets buckets.Store, encryptedBlockSize int32, redundancy eestream.RedundancyStrategy, segmentsSize int64) *Project {
	return &Project{
		buckets:            buckets,
		encryptedBlockSize: encryptedBlockSize,
		redundancy:         redundancy,
		segmentsSize:       segmentsSize,
	}
}
