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
		for i := 0; i < days; i++ {
			h := testrand.Intn(24)
			intervalEndTime := startDate.Add(time.Hour * 24 * time.Duration(i)).Add(time.Hour * time.Duration(h))
			stamp := storageusage.Stamp{
				SatelliteID:     satellite,
				AtRestTotal:     math.Round(testrand.Float64n(1000)),
				IntervalStart:   time.Date(intervalEndTime.Year(), intervalEndTime.Month(), intervalEndTime.Day(), 0, 0, 0, 0, intervalEndTime.Location()),
				IntervalEndTime: intervalEndTime,
			}
			stamps = append(stamps, stamp)
		}
	}

	return stamps
}
