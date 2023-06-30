// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/storj/satellite/nodeselection/uploadselection"
)

// PlacementRules can crate filter based on the placement identifier.
type PlacementRules func(constraint storj.PlacementConstraint) (filter uploadselection.NodeFilters)

// ConfigurablePlacementRule can include the placement definitions for each known identifier.
type ConfigurablePlacementRule struct {
	placements map[storj.PlacementConstraint]uploadselection.NodeFilters
}

// NewPlacementRules creates a fully initialized NewPlacementRules.
func NewPlacementRules() *ConfigurablePlacementRule {
	return &ConfigurablePlacementRule{
		placements: map[storj.PlacementConstraint]uploadselection.NodeFilters{},
	}
}

// AddLegacyStaticRules initializes all the placement rules defined earlier in static golang code.
func (d *ConfigurablePlacementRule) AddLegacyStaticRules() {
	d.placements[storj.EEA] = uploadselection.NodeFilters{}.WithCountryFilter(func(isoCountryCode location.CountryCode) bool {
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
	d.placements[storj.EU] = uploadselection.NodeFilters{}.WithCountryFilter(func(isoCountryCode location.CountryCode) bool {
		for _, c := range location.EuCountries {
			if c == isoCountryCode {
				return true
			}
		}
		return false
	})
	d.placements[storj.US] = uploadselection.NodeFilters{}.WithCountryFilter(func(isoCountryCode location.CountryCode) bool {
		return isoCountryCode == location.UnitedStates
	})
	d.placements[storj.DE] = uploadselection.NodeFilters{}.WithCountryFilter(func(isoCountryCode location.CountryCode) bool {
		return isoCountryCode == location.Germany
	})
	d.placements[storj.NR] = uploadselection.NodeFilters{}.WithCountryFilter(func(isoCountryCode location.CountryCode) bool {
		return isoCountryCode != location.Russia && isoCountryCode != location.Belarus
	})
}

// AddPlacementRule registers a new placement.
func (d *ConfigurablePlacementRule) AddPlacementRule(id storj.PlacementConstraint, filters uploadselection.NodeFilters) {
	d.placements[id] = filters
}

// CreateFilters implements PlacementCondition.
func (d *ConfigurablePlacementRule) CreateFilters(constraint storj.PlacementConstraint) (filter uploadselection.NodeFilters) {
	if constraint == 0 {
		return uploadselection.NodeFilters{}
	}
	if filters, found := d.placements[constraint]; found {
		return filters
	}
	return uploadselection.ExcludeAll
}

// CreateDefaultPlacementRules returns with a default set of configured placement rules.
func CreateDefaultPlacementRules(satelliteID storj.NodeID) PlacementRules {
	placement := NewPlacementRules()
	placement.AddLegacyStaticRules()
	placement.AddPlacementRule(10, uploadselection.NodeFilters{
		uploadselection.NewTagFilter(satelliteID, "selection", []byte("true")),
	})
	return placement.CreateFilters
}
