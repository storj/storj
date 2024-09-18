// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"storj.io/common/storj"
	"storj.io/storj/satellite/buckets"
)

// Placement contains placement info.
type Placement struct {
	DefaultPlacement storj.PlacementConstraint `json:"defaultPlacement"`
	Location         string                    `json:"location"`
}

// BucketMetadata contains bucket name, versioning and placement info.
type BucketMetadata struct {
	Name              string             `json:"name"`
	Versioning        buckets.Versioning `json:"versioning"`
	Placement         Placement          `json:"placement"`
	ObjectLockEnabled bool               `json:"objectLockEnabled"`
}
