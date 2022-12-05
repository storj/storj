// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedbtest

import (
	"math"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/storageusage"
)

// MakeStorageUsageStamps creates storage usage stamps and expected summaries for provided satellites.
// Creates one entry per day for 30 days with last date as beginning of provided endDate.
func MakeStorageUsageStamps(satellites []storj.NodeID, days int, endDate time.Time) []storageusage.Stamp {
	var stamps []storageusage.Stamp

	startDate := time.Date(endDate.Year(), endDate.Month(), endDate.Day()-days, 0, 0, 0, 0, endDate.Location())
	for _, satellite := range satellites {
		previousStampIntervalEndTime := startDate
		for i := 0; i < days; i++ {
			h := testrand.Intn(24)
			intervalEndTime := startDate.Add(time.Hour * 24 * time.Duration(i)).Add(time.Hour * time.Duration(h))
			atRestTotalBytes := math.Round(testrand.Float64n(100))
			intervalInHours := float64(24)
			if i > 0 {
				intervalInHours = intervalEndTime.Sub(previousStampIntervalEndTime).Hours()
			}
			previousStampIntervalEndTime = intervalEndTime
			stamp := storageusage.Stamp{
				SatelliteID:      satellite,
				AtRestTotalBytes: atRestTotalBytes,
				AtRestTotal:      atRestTotalBytes * intervalInHours,
				IntervalInHours:  intervalInHours,
				IntervalStart:    time.Date(intervalEndTime.Year(), intervalEndTime.Month(), intervalEndTime.Day(), 0, 0, 0, 0, intervalEndTime.Location()),
				IntervalEndTime:  intervalEndTime,
			}
			stamps = append(stamps, stamp)
		}
	}

	return stamps
}
