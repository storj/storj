// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"storj.io/storj/internal/memory"
)

const (
	// AverageDaysInMonth is how many days in a month
	AverageDaysInMonth = 30
)

// ExceedsAlphaUsage returns true if more than 25GB of storage is currently in use
// or if 25GB of bandwidth or has been used in the past month (30 days)
// TODO(jg): remove this code once we no longer need usage limiting for alpha release
// Ref: https://storjlabs.atlassian.net/browse/V3-1274
func ExceedsAlphaUsage(bandwidthGetTotal, inlineTotal, remoteTotal int64, maxAlphaUsageGB memory.Size) (bool, string) {
	if bandwidthGetTotal >= maxAlphaUsageGB.Int64() {
		return true, "bandwidth"
	}

	if inlineTotal+remoteTotal >= maxAlphaUsageGB.Int64() {
		return true, "storage"
	}

	return false, ""
}
