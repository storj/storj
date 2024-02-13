// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"storj.io/common/storj"
)

// Placement contains placement info.
type Placement struct {
	DefaultPlacement storj.PlacementConstraint `json:"defaultPlacement"`
	Location         string                    `json:"location"`
}

// BucketPlacement contains bucket name and placement info.
type BucketPlacement struct {
	Name      string    `json:"name"`
	Placement Placement `json:"placement"`
}
