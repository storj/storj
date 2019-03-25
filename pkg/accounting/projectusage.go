// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"storj.io/storj/internal/memory"
)

// ExceedsAlphaUsage returns true if more than 25GB of storage or 25 GBh of bandwidth or has been used in the past month
// TODO: remove this code once we no longer need usage limiting for alpha release
// Ref: https://storjlabs.atlassian.net/browse/V3-1274
func ExceedsAlphaUsage(bandwidthGetTotal, inlineTotal, remoteTotal uint64, maxAlphaUsageGB memory.Size) bool {
	const (
		hoursInDay     = 24
		avgDaysInMonth = 30
	)

	GBh := maxAlphaUsageGB * avgDaysInMonth * hoursInDay
	if bandwidthGetTotal >= uint64(GBh) {
		return true
	}

	if inlineTotal+remoteTotal >= uint64(maxAlphaUsageGB) {
		return true
	}

	return false
}
