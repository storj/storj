// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"storj.io/storj/uplink/metainfo"
)

// Project implements project management operations
type Project struct {
	metainfoClient metainfo.Client
}

// NewProject constructs a *Project
func NewProject(client metainfo.Client) *Project {
	return &Project{
		metainfoClient: client,
	}
}
