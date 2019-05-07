// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"storj.io/storj/internal/memory"
)

const (
	// AverageDaysInMonth is how many days in a month
	AverageDaysInMonth = 30
	// ExpansionFactor is the expansion for redundancy, based on the default
	// redundancy scheme for the uplink.
	ExpansionFactor = 3
)

// ExceedsUsageLimit returns true if the storage or bandwidth usage limits have been exceeded
// for a project in the past month (30 days). The usage limit is 25GB multiplied by the redundancy
// expansion factor, so that the uplinks have a raw limit of 25GB.
// Ref: https://storjlabs.atlassian.net/browse/V3-1274
func ExceedsUsageLimit(bandwidthGetTotal, inlineTotal, remoteTotal int64, maxAlphaUsageGB memory.Size) (bool, string) {
	maxUsage := maxAlphaUsageGB.Int64() * int64(ExpansionFactor)
	if bandwidthGetTotal >= maxUsage {
		return true, "bandwidth"
	}

	if inlineTotal+remoteTotal >= maxUsage {
		return true, "storage"
	}

	return false, ""
}
