// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package entitlements

import "storj.io/common/storj"

// ProductPlacementMappings maps product IDs to their corresponding placement constraints.
type ProductPlacementMappings map[int32][]storj.PlacementConstraint

// ProjectFeatures defines the features available for a project.
type ProjectFeatures struct {
	NewBucketPlacements      []storj.PlacementConstraint `json:"new_bucket_placements"`
	ProductPlacementMappings ProductPlacementMappings    `json:"product_placement_mappings"`
}
