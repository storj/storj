// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"storj.io/common/storj"
)

// Placement contains placement info.
type Placement struct {
	ID       storj.PlacementConstraint `json:"id"`
	Location string                    `json:"location"`
}
