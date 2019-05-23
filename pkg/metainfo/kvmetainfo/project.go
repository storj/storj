// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/uplink/metainfo"
)

// Project implements project management operations
type Project struct {
	metainfoClient metainfo.Client
	redundancy     eestream.RedundancyStrategy
	segmentsSize   int64
}

// NewProject constructs a *Project
func NewProject(client metainfo.Client, redundancy eestream.RedundancyStrategy, segmentsSize int64) *Project {
	return &Project{
		metainfoClient: client,
		redundancy:     redundancy,
		segmentsSize:   segmentsSize,
	}
}
