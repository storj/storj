// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jtolio/mito"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"

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

// String implements pflag.Value.
func (d *ConfigurablePlacementRule) String() string {
	parts := []string{}
	for id, filter := range d.placements {
		// we can hide the internal rules...
		if id > 9 {
			// TODO: we need proper String implementation for all the used filters
			parts = append(parts, fmt.Sprintf("%d:%s", id, filter))
		}
	}
	return strings.Join(parts, ";")
}

// Set implements pflag.Value.
func (d *ConfigurablePlacementRule) Set(s string) error {
	if d.placements == nil {
		d.placements = make(map[storj.PlacementConstraint]nodeselection.NodeFilters)
	}
	d.AddLegacyStaticRules()
	return d.AddPlacementFromString(s)
}

// Type implements pflag.Value.
func (d *ConfigurablePlacementRule) Type() string {
	return "placement-rule"
}

var _ pflag.Value = &ConfigurablePlacementRule{}

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

// AddPlacementFromString parses placement definition form string representations from id:definition;id:definition;...
func (d *ConfigurablePlacementRule) AddPlacementFromString(definitions string) error {
	env := map[any]any{
		"country": func(countries ...string) (nodeselection.NodeFilters, error) {
			countryCodes := make([]location.CountryCode, len(countries))
			for i, country := range countries {
				countryCodes[i] = location.ToCountryCode(country)
			}
			return nodeselection.NodeFilters{}.WithCountryFilter(func(code location.CountryCode) bool {
				for _, expectedCode := range countryCodes {
					if code == expectedCode {
						return true
					}
				}
				return false
			}), nil
		},
		"all": func(filters ...nodeselection.NodeFilters) (nodeselection.NodeFilters, error) {
			res := nodeselection.NodeFilters{}
			for _, filter := range filters {
				res = append(res, filter...)
			}
			return res, nil
		},
		"tag": func(nodeIDstr string, key string, value any) (nodeselection.NodeFilters, error) {
			nodeID, err := storj.NodeIDFromString(nodeIDstr)
			if err != nil {
				return nil, err
			}
			var rawValue []byte
			switch v := value.(type) {
			case string:
				rawValue = []byte(v)
			case []byte:
				rawValue = v
			default:
				return nil, errs.New("3rd argument of tag() should be string or []byte")
			}
			res := nodeselection.NodeFilters{
				nodeselection.NewTagFilter(nodeID, key, rawValue),
			}
			return res, nil
		},
	}
	for _, definition := range strings.Split(definitions, ";") {
		definition = strings.TrimSpace(definition)
		if definition == "" {
			continue
		}
		idDef := strings.SplitN(definition, ":", 2)

		val, err := mito.Eval(idDef[1], env)
		if err != nil {
			return errs.Wrap(err)
		}
		id, err := strconv.Atoi(idDef[0])
		if err != nil {
			return errs.Wrap(err)
		}
		d.placements[storj.PlacementConstraint(id)] = val.(nodeselection.NodeFilters)
	}
	return nil
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
