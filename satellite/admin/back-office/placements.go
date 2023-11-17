// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"

	"storj.io/common/storj"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/nodeselection"
)

// PlacementInfo contains the ID and location of a placement rule.
type PlacementInfo struct {
	ID       storj.PlacementConstraint `json:"id"`
	Location string                    `json:"location"`
}

// GetPlacements returns IDs and locations of placement rules.
func (s *Server) GetPlacements(ctx context.Context) ([]PlacementInfo, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	placements := s.placement.SupportedPlacements()
	infos := make([]PlacementInfo, 0, len(placements))
	for _, placement := range placements {
		filter := s.placement.CreateFilters(placement)
		infos = append(infos, PlacementInfo{
			ID:       placement,
			Location: nodeselection.GetAnnotation(filter, nodeselection.Location),
		})
	}

	return infos, api.HTTPError{}
}
