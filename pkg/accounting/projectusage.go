// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

const (
	// AvgDaysInMonth is how many days in a month
	AvgDaysInMonth = 30
	hoursInDay     = 24
)

// ExceedsAlphaUsage returns true if more than 25GB of storage or 25 GBh of bandwidth or has been used in the past month
// TODO: remove this code once we no longer need usage limiting for alpha release
// Ref: https://storjlabs.atlassian.net/browse/V3-1274
func ExceedsAlphaUsage(bandwidthGetTotal, inlineTotal, remoteTotal uint64, maxAlphaUsageGB int64) (bool, string) {
	if bandwidthGetTotal >= uint64(maxAlphaUsageGB) {
		return true, "bandwidth"
	}

	if inlineTotal+remoteTotal >= uint64(maxAlphaUsageGB) {
		return true, "storage"
	}

	return false, ""
}
