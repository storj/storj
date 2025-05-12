// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// PlacementConfig is placement configuration, separated from other generic configuration.
type PlacementConfig struct {
	Placement ConfigurablePlacementRule `help:"detailed placement rules in the form 'id:definition;id:definition;...' where id is a 16 bytes integer (use >10 for backward compatibility), definition is a combination of the following functions:country(2 letter country codes,...), tag(nodeId, key, bytes(value)) all(...,...)."`
}

// Module is a mud module.
func Module(ball *mud.Ball) {
	// TODO: use trackers when we need them...
	mud.Provide[PlacementConfigEnvironment](ball, func() PlacementConfigEnvironment {
		return NewPlacementConfigEnvironment(nil, nil)
	})
	mud.View[PlacementDefinitions, PlacementRules](ball, func(p PlacementDefinitions) PlacementRules {
		return p.CreateFilters
	})
	config.RegisterConfig[PlacementConfig](ball, "")
}
