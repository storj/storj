// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/jtolio/mito"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/shared/location"
)

// Placement defined all the custom behavior metadata of a specific placement group.
type Placement struct {
	// the unique ID of the placement
	ID storj.PlacementConstraint
	// meaningful identifier/label for Humans. Will be used on UI.
	Name string
	// binding condition for filtering out nodes
	NodeFilter NodeFilter
	// binding condition for filtering out nodes, but only during uploads. Repair won't move nodes based on this.
	UploadFilter NodeFilter
	// Selector is the method how the nodes are selected from the full node space (eg. pick a subnet first, and pick a node from the subnet)
	Selector NodeSelectorInit
	// checked by repair job, applied to the full selection. Out of placement items will be replaced by new, selected by the Selector.
	Invariant Invariant
	// DownloadSelector is the method for how the nodes are selected for
	// downloading (e.g., at random from the uploaded set, or filtered down with
	// choice of 2).
	DownloadSelector DownloadSelector

	// CohortRequirements, if set, specify how the uplink will determine if
	// enough pieces with the right rules have been uploaded.
	CohortRequirements *CohortRequirements
	// CohortNames, if set, specifies how to calculate cohort names from a given
	// SelectedNode, for return in the AddressedOrderLimits.
	CohortNames map[string]CohortName

	// EC defines erasure coding parameter overrides.
	EC ECParameters `yaml:"ec"`
}

// ECParameters can be used to override certain part of the RS parameters.
type ECParameters struct {
	Minimum int
	Success func(k int) int
	Total   int
	Repair  func(k int) int
}

// UnmarshalYAML handles YAML unmarshaling for ECParameters.
func (e *ECParameters) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {
	// First try to unmarshal as a struct with mixed repair value (int or string)
	type ECParametersYAML struct {
		Minimum int `yaml:"minimum"`
		Success any `yaml:"success"`
		Total   int `yaml:"total"`
		Repair  any `yaml:"repair"`
	}

	var params ECParametersYAML
	if err := unmarshal(&params); err != nil {
		return err
	}

	e.Minimum = params.Minimum
	e.Total = params.Total

	if params.Success != nil {
		e.Success, err = parseRedundancyValue(params.Minimum, params.Success)
		if err != nil {
			return err
		}
	}
	if params.Repair != nil {
		e.Repair, err = parseRedundancyValue(params.Minimum, params.Repair)
		if err != nil {
			return err
		}
	}

	return nil
}

func parseRedundancyValue(minimum int, value any) (func(k int) int, error) {
	switch val := value.(type) {
	case int:
		// Static repair value
		if val > 0 {
			staticVal := val
			return func(k int) int {
				if k == minimum {
					return staticVal
				}
				// if the repair is static, but defined for a different k, don't use it.
				// it's possible that repair will try to get repair threshold for different k
				return 0
			}, nil
		}
	case string:
		// Dynamic repair value (e.g., "+5" means k+5)
		if strings.HasPrefix(val, "+") {
			offsetStr := val[1:]
			offset, err := strconv.Atoi(offsetStr)
			if err != nil {
				return nil, errs.New("invalid EC parameter offset value '%s': %v", val, err)
			}
			return func(k int) int {
				return k + offset
			}, nil
		} else {
			return nil, errs.New("unsupported EC parameter string format '%s', expected format like '+5'", val)
		}
	}
	return nil, errs.New("EC fields must be int or string, got %T", value)
}

// Match implements NodeFilter.
func (p Placement) Match(node *SelectedNode) bool {
	return p.NodeFilter.Match(node)
}

// MatchForUpload implements NodeFilter. It checks not just the global filter but also the filter for uploads.
func (p Placement) MatchForUpload(node *SelectedNode) bool {
	return p.NodeFilter.Match(node) && p.UploadFilter.Match(node)
}

// GetAnnotation implements NodeFilterWithAnnotation.
// Deprecated: use Name instead.
func (p Placement) GetAnnotation(name string) string {
	if name == Location && p.Name != "" {
		return p.Name
	}
	switch filter := p.NodeFilter.(type) {
	case NodeFilterWithAnnotation:
		return filter.GetAnnotation(name)
	default:
		return ""
	}
}

var _ NodeFilter = Placement{}

var _ NodeFilterWithAnnotation = Placement{}

// NodeSelectorInit initializes a stateful NodeSelector when node cache is refreshed.
type NodeSelectorInit func(context.Context, []*SelectedNode, NodeFilter) NodeSelector

// NodeSelector pick random nodes based on a specific algorithm.
// Nodes from excluded should never be used. Same is true for alreadySelected, but it may also trigger other restrictions
// (for example, when a last_net is already selected, all the nodes from the same net should be excluded as well.
type NodeSelector func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*SelectedNode) ([]*SelectedNode, error)

// ErrPlacement is used for placement definition related parsing errors.
var ErrPlacement = errs.Class("placement")

// PlacementRules can crate filter based on the placement identifier.
type PlacementRules func(constraint storj.PlacementConstraint) (filter NodeFilter, selector DownloadSelector)

// PlacementDefinitions can include the placement definitions for each known identifier.
type PlacementDefinitions map[storj.PlacementConstraint]Placement

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
// defaultPlacement is used to create the placement if no placement has been set.
func (c ConfigurablePlacementRule) Parse(defaultPlacement func() (Placement, error), environment PlacementConfigEnvironment) (PlacementDefinitions, error) {
	if environment == nil {
		environment = NewPlacementConfigEnvironment(nil, nil)
	}
	if c.PlacementRules == "" {
		dp, err := defaultPlacement()
		if err != nil {
			return PlacementDefinitions{}, err
		}
		pdef := NewPlacementDefinitions(dp)
		pdef.AddLegacyStaticRules()
		return pdef, nil
	}
	rules := c.PlacementRules
	if _, err := os.Stat(rules); err == nil {
		if strings.HasSuffix(rules, ".yaml") {
			// new style of config, all others are deprecated
			return LoadConfig(rules, environment)

		}
		ruleBytes, err := os.ReadFile(rules)
		if err != nil {
			return nil, ErrPlacement.New("Placement definition file couldn't be read: %s %v", rules, err)
		}
		rules = string(ruleBytes)
	}
	if strings.HasPrefix(rules, "/") || strings.HasPrefix(rules, "./") || strings.HasPrefix(rules, "../") {
		return nil, ErrPlacement.New("Placement definition (%s) looks to be a path, but file doesn't exist at that place", rules)
	}
	d := PlacementDefinitions(map[storj.PlacementConstraint]Placement{})
	d.AddLegacyStaticRules()
	err := d.AddPlacementFromString(rules)
	return d, err
}

var _ pflag.Value = &ConfigurablePlacementRule{}

// TestPlacementDefinitions creates placements for testing. Only 0 placement is defined with subnetfiltering.
func TestPlacementDefinitions() PlacementDefinitions {
	return map[storj.PlacementConstraint]Placement{
		storj.DefaultPlacement: {
			ID:               storj.DefaultPlacement,
			NodeFilter:       AnyFilter{},
			Selector:         AttributeGroupSelector(LastNetAttribute),
			Invariant:        ClumpingByAttribute(LastNetAttribute, 1),
			DownloadSelector: DefaultDownloadSelector,
		},
	}
}

// TestPlacementDefinitionsWithFraction creates placements for testing. Similar to TestPlacementDefinitions, but also selects newNodes based on fraction.
func TestPlacementDefinitionsWithFraction(newNodeFraction float64) PlacementDefinitions {
	return map[storj.PlacementConstraint]Placement{
		storj.DefaultPlacement: {
			ID:               storj.DefaultPlacement,
			NodeFilter:       AnyFilter{},
			Selector:         UnvettedSelector(newNodeFraction, AttributeGroupSelector(LastNetAttribute)),
			DownloadSelector: DefaultDownloadSelector,
		},
	}
}

// NewPlacementDefinitions creates a PlacementDefinition with a default placement.
func NewPlacementDefinitions(placements ...Placement) PlacementDefinitions {
	result := map[storj.PlacementConstraint]Placement{}
	for _, p := range placements {
		result[p.ID] = p
	}
	return result
}

// AddLegacyStaticRules initializes all the placement rules defined earlier in static golang code.
func (d PlacementDefinitions) AddLegacyStaticRules() {
	d[storj.EEA] = Placement{
		NodeFilter:       NodeFilters{NewCountryFilter(location.NewSet(EeaCountriesWithoutEu...).With(EuCountries...))},
		DownloadSelector: DefaultDownloadSelector,
	}
	d[storj.EU] = Placement{
		NodeFilter:       NodeFilters{NewCountryFilter(location.NewSet(EuCountries...))},
		DownloadSelector: DefaultDownloadSelector,
	}
	d[storj.US] = Placement{
		NodeFilter:       NodeFilters{NewCountryFilter(location.NewSet(location.UnitedStates))},
		DownloadSelector: DefaultDownloadSelector,
	}
	d[storj.DE] = Placement{
		NodeFilter:       NodeFilters{NewCountryFilter(location.NewSet(location.Germany))},
		DownloadSelector: DefaultDownloadSelector,
	}
	d[storj.NR] = Placement{
		NodeFilter:       NodeFilters{NewCountryFilter(location.NewFullSet().Without(location.Russia, location.Belarus, location.None))},
		DownloadSelector: DefaultDownloadSelector,
	}
}

// AddPlacement registers a new placement.
func (d PlacementDefinitions) AddPlacement(id storj.PlacementConstraint, placement Placement) {
	d[id] = placement
}

// AddPlacementRule registers a new placement.
func (d PlacementDefinitions) AddPlacementRule(id storj.PlacementConstraint, filter NodeFilter, downloadSelector DownloadSelector) {
	placement := Placement{
		NodeFilter:       filter,
		Selector:         AttributeGroupSelector(LastNetAttribute),
		Invariant:        ClumpingByAttribute(LastNetAttribute, 1),
		DownloadSelector: downloadSelector,
	}
	if GetAnnotation(filter, AutoExcludeSubnet) == AutoExcludeSubnetOFF {
		placement.Selector = RandomSelector()
	}
	d[id] = placement
}

type stringNotMatch string

// AddPlacementFromString parses placement definition form string representations from id:definition;id:definition;...
// Deprecated: we will switch to the YAML based configuration.
func (d PlacementDefinitions) AddPlacementFromString(definitions string) error {
	env := map[any]any{
		"country": func(countries ...string) (NodeFilter, error) {
			return NewCountryFilterFromString(countries)
		},
		"placement": func(ix int64) (NodeFilter, error) {
			filter, found := d[storj.PlacementConstraint(ix)]
			if !found {
				return nil, ErrPlacement.New("Placement %d is referenced before defined. Please define it first!", ix)
			}
			return filter.NodeFilter, nil
		},
		"all": func(filters ...NodeFilter) (NodeFilters, error) {
			res := NodeFilters{}
			for _, filter := range filters {
				res = append(res, filter)
			}
			return res, nil
		},
		mito.OpAnd: func(env map[any]any, a, b any) (any, error) {
			filter1, ok1 := a.(NodeFilter)
			filter2, ok2 := b.(NodeFilter)
			if !ok1 || !ok2 {
				return nil, ErrPlacement.New("&& is supported only between NodeFilter instances")
			}
			res := NodeFilters{filter1, filter2}
			return res, nil
		},
		mito.OpOr: func(env map[any]any, a, b any) (any, error) {
			filter1, ok1 := a.(NodeFilter)
			filter2, ok2 := b.(NodeFilter)
			if !ok1 || !ok2 {
				return nil, errs.New("OR is supported only between NodeFilter instances")
			}
			return OrFilter{filter1, filter2}, nil
		},
		"tag": func(nodeIDstr string, key string, value any) (NodeFilters, error) {
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
			res := NodeFilters{
				NewTagFilter(nodeID, key, rawValue, match),
			}
			return res, nil
		},
		"annotated": func(filter NodeFilter, kv ...Annotation) (AnnotatedNodeFilter, error) {
			return AnnotatedNodeFilter{
				Filter:      filter,
				Annotations: kv,
			}, nil
		},
		"annotation": func(key string, value string) (Annotation, error) {
			return Annotation{
				Key:   key,
				Value: value,
			}, nil
		},
		"exclude": func(filter NodeFilter) (NodeFilter, error) {
			return NewExcludeFilter(filter), nil
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
		placement := Placement{
			NodeFilter: val.(NodeFilter),

			// setting this requires YAML configuration
			DownloadSelector: DefaultDownloadSelector,
		}

		if GetAnnotation(placement.NodeFilter, AutoExcludeSubnet) != AutoExcludeSubnetOFF {
			placement.Selector = AttributeGroupSelector(LastNetAttribute)
			placement.Invariant = ClumpingByAttribute(LastNetAttribute, 1)
		} else {
			placement.Selector = RandomSelector()
		}

		placement.Name = GetAnnotation(placement.NodeFilter, Location)
		placement.ID = storj.PlacementConstraint(id)

		d[storj.PlacementConstraint(id)] = placement
	}
	return nil
}

// CreateFilters implements PlacementCondition.
func (d PlacementDefinitions) CreateFilters(constraint storj.PlacementConstraint) (filter NodeFilter, selector DownloadSelector) {
	if filters, found := d[constraint]; found {
		return filters.NodeFilter, filters.DownloadSelector
	}
	return NodeFilters{
		ExcludeAllFilter{},
	}, ExcludeAllDownloadSelector
}

// SupportedPlacements returns all the IDs, which have associated placement rules.
func (d PlacementDefinitions) SupportedPlacements() (res []storj.PlacementConstraint) {
	for id := range d {
		res = append(res, id)
	}
	return res
}
