// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"time"

	"cloud.google.com/go/spanner"
)

// MaxStalenessFromAOSI creates a timestamp bound based on as of system interval value.
func MaxStalenessFromAOSI(asOfSystemInterval time.Duration) spanner.TimestampBound {
	if asOfSystemInterval != 0 {
		// spanner requires non-negative staleness
		staleness := asOfSystemInterval
		if staleness < 0 {
			staleness *= -1
		}
		return spanner.MaxStaleness(staleness)
	}
	return spanner.StrongRead()
}
