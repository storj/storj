// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

// DiskSpaceInfo stores all info about storagenode disk space usage
type DiskSpaceInfo struct {
	Used      float64 `json:"used"`
	Available float64 `json:"available"`
}
