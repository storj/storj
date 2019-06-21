// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

// DiskSpaceInfo stores all info about storagenode disk space usage
type DiskSpaceInfo struct {
	Available int64 `json:"available"`
	Used      int64 `json:"used"`
}
