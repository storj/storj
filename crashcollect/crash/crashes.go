// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package crash

import (
	"time"

	"storj.io/common/storj"
)

// Crash holds information about storagenode crash.
type Crash struct {
	ID              storj.NodeID
	CompressedPanic []byte
	CrashedAt       time.Time
}
