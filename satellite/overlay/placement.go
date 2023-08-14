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
type PlacementRules func(constraint storj.PlacementConstraint) (filter nodeselection.NodeFilter)

// ConfigurablePlacementRule can include the placement definitions for each known identifier.
type ConfigurablePlacementRule struct {
	placements map[storj.PlacementConstraint]nodeselection.NodeFilter
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
		d.placements = make(map[storj.PlacementConstraint]nodeselection.NodeFilter)
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
		placements: make(map[storj.PlacementConstraint]nodeselection.NodeFilter),
	}
}

// AddLegacyStaticRules initializes all the placement rules defined earlier in static golang code.
func (d *ConfigurablePlacementRule) AddLegacyStaticRules() {
	d.placements[storj.EEA] = nodeselection.NodeFilters{nodeselection.NewCountryFilter(nodeselection.EeaCountries)}
	d.placements[storj.EU] = nodeselection.NodeFilters{nodeselection.NewCountryFilter(nodeselection.EuCountries)}
	d.placements[storj.US] = nodeselection.NodeFilters{nodeselection.NewCountryFilter(location.NewSet(location.UnitedStates))}
	d.placements[storj.DE] = nodeselection.NodeFilters{nodeselection.NewCountryFilter(location.NewSet(location.Germany))}
	d.placements[storj.NR] = nodeselection.NodeFilters{nodeselection.NewCountryFilter(location.NewFullSet().Without(location.Russia, location.Belarus, location.None))}
}

// AddPlacementRule registers a new placement.
func (d *ConfigurablePlacementRule) AddPlacementRule(id storj.PlacementConstraint, filter nodeselection.NodeFilter) {
	d.placements[id] = filter
}

// AddPlacementFromString parses placement definition form string representations from id:definition;id:definition;...
func (d *ConfigurablePlacementRule) AddPlacementFromString(definitions string) error {
	env := map[any]any{
		"country": func(countries ...string) (nodeselection.NodeFilters, error) {
			var set location.Set
			for _, country := range countries {
				code := location.ToCountryCode(country)
				if code == location.None {
					return nil, errs.New("invalid country code %q", code)
				}
				set.Include(code)
			}
			return nodeselection.NodeFilters{nodeselection.NewCountryFilter(set)}, nil
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
		"annotated": func(filter nodeselection.NodeFilter, kv map[string]string) (nodeselection.AnnotatedNodeFilter, error) {
			return nodeselection.AnnotatedNodeFilter{
				Filter:      filter,
				Annotations: kv,
			}, nil
		},
		"annotation": func(key string, value string) (map[string]string, error) {
			return map[string]string{
				key: value,
			}, nil
		},
		"exclude": func(filter nodeselection.NodeFilter) (nodeselection.NodeFilter, error) {
			return nodeselection.NewExcludeFilter(filter), nil
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
		d.placements[storj.PlacementConstraint(id)] = val.(nodeselection.NodeFilter)
	}
	return nil
}

// CreateFilters implements PlacementCondition.
func (d *ConfigurablePlacementRule) CreateFilters(constraint storj.PlacementConstraint) (filter nodeselection.NodeFilter) {
	if constraint == storj.EveryCountry {
		return nodeselection.NodeFilters{}
	}
	if filters, found := d.placements[constraint]; found {
		return filters
	}
	return nodeselection.NodeFilters{
		nodeselection.ExcludeAllFilter{},
	}
}
