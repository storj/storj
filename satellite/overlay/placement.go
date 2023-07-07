// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/storj/satellite/nodeselection"
)

// PlacementRules can crate filter based on the placement identifier.
type PlacementRules func(constraint storj.PlacementConstraint) (filter nodeselection.NodeFilters)

// ConfigurablePlacementRule can include the placement definitions for each known identifier.
type ConfigurablePlacementRule struct {
	placements map[storj.PlacementConstraint]nodeselection.NodeFilters
}

// NewPlacementRules creates a fully initialized NewPlacementRules.
func NewPlacementRules() *ConfigurablePlacementRule {
	return &ConfigurablePlacementRule{
		placements: map[storj.PlacementConstraint]nodeselection.NodeFilters{},
	}
}

// AddLegacyStaticRules initializes all the placement rules defined earlier in static golang code.
func (d *ConfigurablePlacementRule) AddLegacyStaticRules() {
	d.placements[storj.EEA] = nodeselection.NodeFilters{}.WithCountryFilter(func(isoCountryCode location.CountryCode) bool {
		for _, c := range location.EeaNonEuCountries {
			if c == isoCountryCode {
				return true
			}
		}
		for _, c := range location.EuCountries {
			if c == isoCountryCode {
				return true
			}
		}
		return false
	})
	d.placements[storj.EU] = nodeselection.NodeFilters{}.WithCountryFilter(func(isoCountryCode location.CountryCode) bool {
		for _, c := range location.EuCountries {
			if c == isoCountryCode {
				return true
			}
		}
		return false
	})
	d.placements[storj.US] = nodeselection.NodeFilters{}.WithCountryFilter(func(isoCountryCode location.CountryCode) bool {
		return isoCountryCode == location.UnitedStates
	})
	d.placements[storj.DE] = nodeselection.NodeFilters{}.WithCountryFilter(func(isoCountryCode location.CountryCode) bool {
		return isoCountryCode == location.Germany
	})
	d.placements[storj.NR] = nodeselection.NodeFilters{}.WithCountryFilter(func(isoCountryCode location.CountryCode) bool {
		return isoCountryCode != location.Russia && isoCountryCode != location.Belarus
	})
}

// AddPlacementRule registers a new placement.
func (d *ConfigurablePlacementRule) AddPlacementRule(id storj.PlacementConstraint, filters nodeselection.NodeFilters) {
	d.placements[id] = filters
}

// CreateFilters implements PlacementCondition.
func (d *ConfigurablePlacementRule) CreateFilters(constraint storj.PlacementConstraint) (filter nodeselection.NodeFilters) {
	if constraint == 0 {
		return nodeselection.NodeFilters{}
	}
	if filters, found := d.placements[constraint]; found {
		return filters
	}
	return nodeselection.ExcludeAll
}

// CreateDefaultPlacementRules returns with a default set of configured placement rules.
func CreateDefaultPlacementRules(satelliteID storj.NodeID) PlacementRules {
	placement := NewPlacementRules()
	placement.AddLegacyStaticRules()
	placement.AddPlacementRule(10, nodeselection.NodeFilters{
		nodeselection.NewTagFilter(satelliteID, "selection", []byte("true")),
	})
	return placement.CreateFilters
}
