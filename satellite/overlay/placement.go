// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"os"
	"sort"
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

// PlacementDefinitions can include the placement definitions for each known identifier.
type PlacementDefinitions struct {
	placements map[storj.PlacementConstraint]nodeselection.NodeFilter
}

// ConfigurablePlacementRule is a string configuration includes all placement rules in the form of id1:def1,id2:def2...
type ConfigurablePlacementRule struct {
	PlacementRules string
}

// String implements pflag.Value.
func (c *ConfigurablePlacementRule) String() string {
	return c.PlacementRules
}

// Set implements pflag.Value.
func (c *ConfigurablePlacementRule) Set(s string) error {
	c.PlacementRules = s
	return nil
}

// Type implements pflag.Value.
func (c *ConfigurablePlacementRule) Type() string {
	return "configurable-placement-rule"
}

// Parse creates the PlacementDefinitions from the string rules.
func (c ConfigurablePlacementRule) Parse() (*PlacementDefinitions, error) {
	placement := c.PlacementRules
	if _, err := os.Stat(placement); err == nil {
		// it's a a file, let's read it
		raw, err := os.ReadFile(placement)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		placement = string(raw)
	}
	d := NewPlacementDefinitions()
	d.AddLegacyStaticRules()
	err := d.AddPlacementFromString(placement)
	return d, err
}

var _ pflag.Value = &ConfigurablePlacementRule{}

// NewPlacementDefinitions creates a fully initialized NewPlacementDefinitions.
func NewPlacementDefinitions() *PlacementDefinitions {
	return &PlacementDefinitions{
		placements: map[storj.PlacementConstraint]nodeselection.NodeFilter{
			storj.EveryCountry: nodeselection.AnyFilter{}},
	}
}

// AddLegacyStaticRules initializes all the placement rules defined earlier in static golang code.
func (d *PlacementDefinitions) AddLegacyStaticRules() {
	d.placements[storj.EEA] = nodeselection.NodeFilters{nodeselection.NewCountryFilter(location.NewSet(nodeselection.EeaCountriesWithoutEu...).With(nodeselection.EuCountries...))}
	d.placements[storj.EU] = nodeselection.NodeFilters{nodeselection.NewCountryFilter(location.NewSet(nodeselection.EuCountries...))}
	d.placements[storj.US] = nodeselection.NodeFilters{nodeselection.NewCountryFilter(location.NewSet(location.UnitedStates))}
	d.placements[storj.DE] = nodeselection.NodeFilters{nodeselection.NewCountryFilter(location.NewSet(location.Germany))}
	d.placements[storj.NR] = nodeselection.NodeFilters{nodeselection.NewCountryFilter(location.NewFullSet().Without(location.Russia, location.Belarus, location.None))}
}

// AddPlacementRule registers a new placement.
func (d *PlacementDefinitions) AddPlacementRule(id storj.PlacementConstraint, filter nodeselection.NodeFilter) {
	d.placements[id] = filter
}

// AddPlacementFromString parses placement definition form string representations from id:definition;id:definition;...
func (d *PlacementDefinitions) AddPlacementFromString(definitions string) error {
	env := map[any]any{
		"country": func(countries ...string) (nodeselection.NodeFilters, error) {
			var set location.Set
			for _, country := range countries {
				apply := func(modified location.Set, code ...location.CountryCode) location.Set {
					return modified.With(code...)
				}
				if country[0] == '!' {
					apply = func(modified location.Set, code ...location.CountryCode) location.Set {
						return modified.Without(code...)
					}
					country = country[1:]
				}
				switch strings.ToLower(country) {
				case "all", "*", "any":
					set = location.NewFullSet()
				case "none":
					set = apply(set, location.None)
				case "eu":
					set = apply(set, nodeselection.EuCountries...)
				case "eea":
					set = apply(set, nodeselection.EuCountries...)
					set = apply(set, nodeselection.EeaCountriesWithoutEu...)
				default:
					code := location.ToCountryCode(country)
					if code == location.None {
						return nil, errs.New("invalid country code %q", code)
					}
					set = apply(set, code)
				}
			}
			return nodeselection.NodeFilters{nodeselection.NewCountryFilter(set)}, nil
		},
		"placement": func(ix int64) nodeselection.NodeFilter {
			return d.placements[storj.PlacementConstraint(ix)]
		},
		"all": func(filters ...nodeselection.NodeFilters) (nodeselection.NodeFilters, error) {
			res := nodeselection.NodeFilters{}
			for _, filter := range filters {
				res = append(res, filter...)
			}
			return res, nil
		},
		mito.OpAnd: func(env map[any]any, a, b any) (any, error) {
			filter1, ok1 := a.(nodeselection.NodeFilter)
			filter2, ok2 := b.(nodeselection.NodeFilter)
			if !ok1 || !ok2 {
				return nil, errs.New("&& is supported only between NodeFilter instances")
			}
			res := nodeselection.NodeFilters{filter1, filter2}
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
		"annotated": func(filter nodeselection.NodeFilter, kv ...nodeselection.Annotation) (nodeselection.AnnotatedNodeFilter, error) {
			return nodeselection.AnnotatedNodeFilter{
				Filter:      filter,
				Annotations: kv,
			}, nil
		},
		"annotation": func(key string, value string) (nodeselection.Annotation, error) {
			return nodeselection.Annotation{
				Key:   key,
				Value: value,
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
func (d *PlacementDefinitions) CreateFilters(constraint storj.PlacementConstraint) (filter nodeselection.NodeFilter) {
	if filters, found := d.placements[constraint]; found {
		return filters
	}
	return nodeselection.NodeFilters{
		nodeselection.ExcludeAllFilter{},
	}
}

// ConfiguredPlacements returns with all the placement IDs which has placement definition.
func (d *PlacementDefinitions) ConfiguredPlacements() (res []storj.PlacementConstraint) {
	for k := range d.placements {
		res = append(res, k)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i] < res[j]
	})
	return res
}
