// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import "storj.io/storj/pkg/storage/buckets"

// Project implements project management operations
type Project struct {
	buckets buckets.Store
}

// NewProject constructs a *Project
func NewProject(buckets buckets.Store) *Project {
	return &Project{buckets: buckets}
}
