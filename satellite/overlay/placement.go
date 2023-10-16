// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"bytes"
	"os"
	"strconv"
	"strings"

	"github.com/jtolio/mito"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/storj/satellite/nodeselection"
)

// ErrPlacement is used for placement definition related parsing errors.
var ErrPlacement = errs.Class("placement")

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
	rules := c.PlacementRules
	if _, err := os.Stat(rules); err == nil {
		ruleBytes, err := os.ReadFile(rules)
		if err != nil {
			return nil, ErrPlacement.New("Placement definition file couldn't be read: %s %v", rules, err)
		}
		rules = string(ruleBytes)
	}
	if strings.HasPrefix(rules, "/") || strings.HasPrefix(rules, "./") || strings.HasPrefix(rules, "../") {
		return nil, ErrPlacement.New("Placement definition (%s) looks to be a path, but file doesn't exist at that place", rules)
	}
	d := NewPlacementDefinitions()
	d.AddLegacyStaticRules()
	err := d.AddPlacementFromString(rules)
	return d, err
}

var _ pflag.Value = &ConfigurablePlacementRule{}

// NewPlacementDefinitions creates a fully initialized NewPlacementDefinitions.
func NewPlacementDefinitions() *PlacementDefinitions {
	return &PlacementDefinitions{
		placements: map[storj.PlacementConstraint]nodeselection.NodeFilter{
			storj.DefaultPlacement: nodeselection.AnyFilter{}},
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

type stringNotMatch string

// AddPlacementFromString parses placement definition form string representations from id:definition;id:definition;...
func (d *PlacementDefinitions) AddPlacementFromString(definitions string) error {
	env := map[any]any{
		"country": func(countries ...string) (nodeselection.NodeFilter, error) {
			return nodeselection.NewCountryFilterFromString(countries)
		},
		"placement": func(ix int64) (nodeselection.NodeFilter, error) {
			filter, found := d.placements[storj.PlacementConstraint(ix)]
			if !found {
				return nil, ErrPlacement.New("Placement %d is referenced before defined. Please define it first!", ix)
			}
			return filter, nil
		},
		"all": func(filters ...nodeselection.NodeFilter) (nodeselection.NodeFilters, error) {
			res := nodeselection.NodeFilters{}
			for _, filter := range filters {
				res = append(res, filter)
			}
			return res, nil
		},
		mito.OpAnd: func(env map[any]any, a, b any) (any, error) {
			filter1, ok1 := a.(nodeselection.NodeFilter)
			filter2, ok2 := b.(nodeselection.NodeFilter)
			if !ok1 || !ok2 {
				return nil, ErrPlacement.New("&& is supported only between NodeFilter instances")
			}
			res := nodeselection.NodeFilters{filter1, filter2}
			return res, nil
		},
		mito.OpOr: func(env map[any]any, a, b any) (any, error) {
			filter1, ok1 := a.(nodeselection.NodeFilter)
			filter2, ok2 := b.(nodeselection.NodeFilter)
			if !ok1 || !ok2 {
				return nil, errs.New("OR is supported only between NodeFilter instances")
			}
			return nodeselection.OrFilter{filter1, filter2}, nil
		},
		"tag": func(nodeIDstr string, key string, value any) (nodeselection.NodeFilters, error) {
			nodeID, err := storj.NodeIDFromString(nodeIDstr)
			if err != nil {
				return nil, err
			}

			var rawValue []byte
			match := bytes.Equal
			switch v := value.(type) {
			case string:
				rawValue = []byte(v)
			case []byte:
				rawValue = v
			case stringNotMatch:
				match = func(a, b []byte) bool {
					return !bytes.Equal(a, b)
				}
				rawValue = []byte(v)
			default:
				return nil, ErrPlacement.New("3rd argument of tag() should be string or []byte")
			}
			res := nodeselection.NodeFilters{
				nodeselection.NewTagFilter(nodeID, key, rawValue, match),
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
		"empty": func() string {
			return ""
		},
		"notEmpty": func() any {
			return stringNotMatch("")
		},
	}
	for _, definition := range strings.Split(definitions, ";") {
		definition = strings.TrimSpace(definition)
		if definition == "" {
			continue
		}
		idDef := strings.SplitN(definition, ":", 2)

		if len(idDef) != 2 {
			return ErrPlacement.New("placement definition should be in the form ID:definition (but it was %s)", definition)
		}
		val, err := mito.Eval(idDef[1], env)
		if err != nil {
			return ErrPlacement.New("Error in line '%s' when placement rule is parsed: %v", idDef[1], err)
		}
		id, err := strconv.Atoi(idDef[0])
		if err != nil {
			return ErrPlacement.Wrap(err)
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

// SupportedPlacements returns all the IDs, which have associated placement rules.
func (d *PlacementDefinitions) SupportedPlacements() (res []storj.PlacementConstraint) {
	for id := range d.placements {
		res = append(res, id)
	}
	return res
}
