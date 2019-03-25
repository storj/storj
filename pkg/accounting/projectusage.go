// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"storj.io/storj/internal/memory"
)

// ExceedsAlphaUsage returns true if more than 25GB of bandwidth or storage usage has been used in the past month
// TODO: remove this code once we no longer need usage limiting for alpha release
// Ref: https://storjlabs.atlassian.net/browse/V3-1274
func ExceedsAlphaUsage(bandwidthPutTotal, bandwidthGetTotal, inlineTotal, remoteTotal uint64, alphaMaxUsage memory.Size) bool {
	if bandwidthPutTotal+bandwidthGetTotal >= uint64(alphaMaxUsage) {
		return true
	}

	if inlineTotal+remoteTotal >= uint64(alphaMaxUsage) {
		return true
	}

	return false
}
