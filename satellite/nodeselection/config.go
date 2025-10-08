// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"

	"github.com/jtolio/mito"
	"github.com/zeebo/errs"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"

	"storj.io/common/storj"
)

func convertToCompareNodes(arg any) (CompareNodes, error) {
	switch a := arg.(type) {
	case CompareNodes:
		return a, nil
	case ScoreNode:
		return Compare(a), nil
	case NodeValue:
		return Compare(ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
			return a(*node)
		})), nil
	}
	return nil, errs.New("argument for choiceofn/choiceoftwo must be CompareNodes, ScoreNode, NodeValue, or UploadSuccessTracker, got %T", arg)
}

// placementConfig is the representation of YAML based placement configuration.
type placementConfig struct {

	// helpers which can be re-used later to simplify config
	Templates map[string]string

	// the placement definitions
	Placements []placementDefinition
}

type placementDefinition struct {
	ID                 storj.PlacementConstraint
	Name               string
	Filter             string
	UploadFilter       string `yaml:"upload-filter"`
	Invariant          string
	Selector           string
	DownloadSelector   string `yaml:"download-selector"`
	CohortRequirements string `yaml:"cohort-requirements"`
	EC                 ECParameters
}

// UploadSuccessTracker can give hints about the frequency of the long-tail cancellation per node.
type UploadSuccessTracker interface {
	// Get gives a Score to the node based on the upload success rate. Can be math.NaN (no information). Higher is better.
	Get(uplink storj.NodeID) func(node *SelectedNode) float64
}

// UploadFailureTracker keeps track of node failures.
type UploadFailureTracker interface {
	Get(node *SelectedNode) float64
}

type uploadFailureTrackerFunc func(node *SelectedNode) float64

func (f uploadFailureTrackerFunc) Get(node *SelectedNode) float64 { return f(node) }

// NoopSuccessTracker doesn't track uploads at all. Always returns with zero.
type NoopSuccessTracker struct {
}

// Get implements UploadSuccessTracker.
func (n NoopSuccessTracker) Get(uplink storj.NodeID) func(node *SelectedNode) float64 {
	return func(node *SelectedNode) float64 { return 0 }
}

var _ UploadSuccessTracker = NoopSuccessTracker{}

// PlacementConfigEnvironment includes all generic functions and variables, which can be used in the configuration.
type PlacementConfigEnvironment map[any]any

// NewPlacementConfigEnvironment creates PlacementConfigEnvironment.
// It initializes the environment with successTracker, failureTracker, and any additional key-value pairs provided in additionalEnv.
func NewPlacementConfigEnvironment(successTracker UploadSuccessTracker, failureTracker UploadFailureTracker) PlacementConfigEnvironment {
	env := make(PlacementConfigEnvironment)

	if successTracker == nil {
		successTracker = NoopSuccessTracker{}
	}
	if failureTracker == nil {
		failureTracker = uploadFailureTrackerFunc(func(node *SelectedNode) float64 { return math.NaN() })
	}

	// Standard trackers
	env["tracker"] = successTracker // backcompat
	env["uploadSuccessTracker"] = successTracker
	env["uploadFailureTracker"] = failureTracker

	return env
}

// AddPrometheusTracker adds a Prometheus tracker to the environment.
func (e PlacementConfigEnvironment) AddPrometheusTracker(tracker any) PlacementConfigEnvironment {
	e["prometheusTracker"] = tracker
	return e
}

func (e PlacementConfigEnvironment) apply(targetEnv map[any]any) {
	for k, v := range e {
		targetEnv[k] = v
	}
}

// LoadConfig loads the placement yaml file and creates the Placement definitions.
func LoadConfig(configFile string, environment PlacementConfigEnvironment) (PlacementDefinitions, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, errs.New("Couldn't read placement config file from %s: %v", configFile, err)
	}
	placements, err := LoadConfigFromString(string(data), environment)
	if err != nil {
		return nil, errs.New("Couldn't parse placement config file from %s: %v", configFile, err)
	}
	return placements, nil
}

// LoadConfigFromString loads the placement YAML from a string and creates the Placement definitions.
func LoadConfigFromString(config string, environment PlacementConfigEnvironment) (PlacementDefinitions, error) {
	placements := make(PlacementDefinitions)

	cfg := &placementConfig{}

	err := yaml.Unmarshal([]byte(config), &cfg)
	if err != nil {
		return placements, errs.New("Couldn't parse placement config as YAML: %v", err)
	}

	templates := map[string]string{}
	for k, v := range cfg.Templates {
		value := v
		for a, b := range cfg.Templates {
			value = strings.ReplaceAll(value, "$"+a, b)
		}
		templates[k] = value
	}

	resolveTemplates := func(orig string) string {
		val := orig
		for k, v := range templates {
			val = strings.ReplaceAll(val, "$"+k, v)
		}
		return val
	}

	for _, def := range cfg.Placements {
		p := Placement{
			ID:   def.ID,
			Name: def.Name,
			EC:   def.EC,
		}

		filter := resolveTemplates(def.Filter)
		p.NodeFilter, err = FilterFromString(filter, environment)
		if err != nil {
			return placements, errs.New("Filter definition %q of placement %d is invalid: %v", filter, def.ID, err)
		}

		uploadFilter := resolveTemplates(def.UploadFilter)
		p.UploadFilter, err = FilterFromString(uploadFilter, environment)
		if err != nil {
			return placements, errs.New("Upload filter definition %q of placement %d is invalid: %v", filter, def.ID, err)
		}

		invariant := resolveTemplates(def.Invariant)
		p.Invariant, err = InvariantFromString(invariant)
		if err != nil {
			return placements, errs.New("Invariant definition %q of placement %d is invalid: %v", invariant, def.ID, err)
		}

		selector := resolveTemplates(def.Selector)
		p.Selector, err = SelectorFromString(selector, environment)
		if err != nil {
			return placements, errs.New("Selector definition %q of placement %d is invalid: %v", selector, def.ID, err)
		}

		downloadSelector := resolveTemplates(def.DownloadSelector)
		p.DownloadSelector, err = DownloadSelectorFromString(downloadSelector, environment)
		if err != nil {
			return placements, errs.New("DownloadSelector definition %q of placement %d is invalid: %v", downloadSelector, def.ID, err)
		}

		cohortRequirements := resolveTemplates(def.CohortRequirements)
		p.CohortRequirements, p.CohortNames, err = CohortRequirementsFromString(cohortRequirements)
		if err != nil {
			return placements, errs.New("CohortRequirements definition %q of placement %d is invalid: %v", cohortRequirements, def.ID, err)
		}

		placements[def.ID] = p
	}
	return placements, nil
}

var supportedFilters = map[any]any{
	"country": func(countries ...string) (NodeFilter, error) {
		return NewCountryFilterFromString(countries)
	},
	"continent": func(continent string) (NodeFilter, error) {
		return NewContinentFilterFromString(continent)
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
	"exclude": func(filter NodeFilter) (NodeFilter, error) {
		return NewExcludeFilter(filter), nil
	},
	"empty": func() string {
		return ""
	},
	"notEmpty": func() any {
		return stringNotMatch("")
	},
	"nodelist": AllowedNodesFromFile,
	"select":   NewAttributeFilter,
	"none": func() NodeFilter {
		return NodeFilterFunc(func(node *SelectedNode) bool {
			return false
		})
	},
	"successfulAtLeastPercent": func(tracker UploadFailureTracker, percent float64) (NodeFilter, error) {
		return NodeFilterFunc(func(node *SelectedNode) bool {
			current := tracker.Get(node)
			return math.IsNaN(current) || percent <= current
		}), nil
	},
}

// FilterFromString parses complex node filter expressions from config lines.
func FilterFromString(expr string, environment PlacementConfigEnvironment) (NodeFilter, error) {
	if expr == "" {
		expr = "all()"
	}
	env := maps.Clone(supportedFilters)
	environment.apply(env)
	filter, err := mito.Eval(expr, env)
	if err != nil {
		return nil, errs.New("Invalid filter definition '%s', %v", expr, err)
	}
	return filter.(NodeFilter), nil
}

// SelectorFromString parses complex node selection rules from config lines.
func SelectorFromString(expr string, environment PlacementConfigEnvironment) (NodeSelectorInit, error) {
	if expr == "" {
		expr = "random()"
	}
	env := map[any]any{
		"attribute": func(attribute interface{}) (NodeSelectorInit, error) {
			switch value := attribute.(type) {
			case NodeAttribute:
				return AttributeGroupSelector(value), nil
			case string:
				attr, err := CreateNodeAttribute(value)
				if err != nil {
					return nil, err
				}
				return AttributeGroupSelector(attr), nil
			default:
				return nil, Error.New("unable to create attribute selector from %s (%T)", expr, attribute)
			}
		},
		"subnet": Subnet,
		"random": func() (NodeSelectorInit, error) {
			return RandomSelector(), nil
		},
		"unvetted": func(newNodeRatio float64, def NodeSelectorInit) (NodeSelectorInit, error) {
			return UnvettedSelector(newNodeRatio, def), nil
		},
		"nodelist": AllowedNodesFromFile,
		"filter":   FilterSelector,
		"compare": func(scoreNodes ...ScoreNode) (CompareNodes, error) {
			return Compare(scoreNodes...), nil
		},
		"choiceofn": func(comparerArg any, n int64, delegate NodeSelectorInit) (NodeSelectorInit, error) {
			comparer, err := convertToCompareNodes(comparerArg)
			if err != nil {
				return nil, errs.Wrap(err)
			}
			return ChoiceOfN(comparer, n, delegate), nil
		},
		"choiceoftwo": func(comparerArg any, delegate NodeSelectorInit) (NodeSelectorInit, error) {
			comparer, err := convertToCompareNodes(comparerArg)
			if err != nil {
				return nil, errs.Wrap(err)
			}
			return ChoiceOfTwo(comparer, delegate), nil
		},
		// DEPRECATED: use choiceoftwo. It's only here for backward-compatibility.
		"pow2": func(comparerArg any, delegate NodeSelectorInit) (NodeSelectorInit, error) {
			comparer, err := convertToCompareNodes(comparerArg)
			if err != nil {
				return nil, errs.Wrap(err)
			}
			return ChoiceOfTwo(comparer, delegate), nil
		},
		"stream": Stream,
		"choiceofns": func(n int64, score any) func(NodeStream) NodeStream {
			score, err := ConvertType(score, reflect.TypeOf(new(ScoreNode)).Elem())
			if err != nil {
				panic(err)
			}
			return ChoiceOfNStream(n, score.(ScoreNode))
		},
		"groupconstraint": GroupConstraint,
		"streamfilter":    StreamFilter,
		"randomstream":    RandomStream,
		"balanced": func(attribute string) (NodeSelectorInit, error) {
			attr, err := CreateNodeAttribute(attribute)
			if err != nil {
				return nil, err
			}
			return BalancedGroupBasedSelector(attr, nil), nil
		},
		"balancedf": func(attribute string, filter NodeFilter) (NodeSelectorInit, error) {
			attr, err := CreateNodeAttribute(attribute)
			if err != nil {
				return nil, err
			}
			return BalancedGroupBasedSelector(attr, filter), nil
		},
		"weighted": func(attribute string, defaultWeight float64, filter NodeFilter) (NodeSelectorInit, error) {
			value, err := CreateNodeValue(attribute)
			if err != nil {
				return nil, err
			}
			return WeightedSelector(NodeValue(func(node SelectedNode) float64 {
				nodeValue := value(node)
				if nodeValue == 0 {
					nodeValue = defaultWeight
				} else if nodeValue <= 0 {
					nodeValue = 0
				}
				return nodeValue
			}), filter), nil
		},
		"weightedf": func(valueFunc NodeValue, filter NodeFilter) (NodeSelectorInit, error) {
			return WeightedSelector(valueFunc, filter), nil
		},
		"weighted_with_adjustment": func(attribute string, defaultWeight, valueBallast, valuePower float64, filter NodeFilter) (NodeSelectorInit, error) {
			value, err := CreateNodeValue(attribute)
			if err != nil {
				return nil, err
			}
			return WeightedSelector(NodeValue(func(node SelectedNode) float64 {
				nodeValue := value(node)
				if nodeValue == 0 {
					nodeValue = defaultWeight
				} else if nodeValue <= 0 {
					nodeValue = 0
				}
				return math.Pow(nodeValue, valuePower) + valueBallast
			}), filter), nil
		},
		"topology":   TopologySelector,
		"filterbest": FilterBest,
		"bestofn":    BestOfN,
		"eq": func(a, b string) func(SelectedNode) bool {
			attr, err := CreateNodeAttribute(a)
			if err != nil {
				return func(SelectedNode) bool { return false }
			}
			return EqualSelector(attr, b)
		},
		"if": func(condition func(SelectedNode) bool, trueAttribute, falseAttribute string) (NodeAttribute, error) {
			trueAttr, err := CreateNodeAttribute(trueAttribute)
			if err != nil {
				return nil, err
			}
			falseAttr, err := CreateNodeAttribute(falseAttribute)
			if err != nil {
				return nil, err
			}
			return IfSelector(condition, trueAttr, falseAttr), nil
		},
		"dual":               DualSelector,
		"choiceofnselection": ChoiceOfNSelection,
		"lastbut":            LastBut,
		"median":             Median,
		"piececount":         PieceCount,
		// deprecated: use * -1 instead
		"desc":           Desc,
		"node_attribute": CreateNodeAttribute,
		"node_value":     CreateNodeValue,
		"maxgroup": func(attribute interface{}) ScoreSelection {
			switch value := attribute.(type) {
			case NodeAttribute:
				return MaxGroup(value)
			case string:
				attr, err := CreateNodeAttribute(value)
				if err != nil {
					panic("Invalid node attribute: " + value + " " + err.Error())
				}
				return MaxGroup(attr)
			default:
				panic(fmt.Sprintf("Argument of maxgroup must be a node attribute (or string), not %T", attribute))
			}
		},
		"atleast": AtLeast,
		"reduce": func(delegate NodeSelectorInit, sortOrder any, needMore ...NeedMore) NodeSelectorInit {
			sorter, err := convertToCompareNodes(sortOrder)
			if err != nil {
				panic(err)
			}
			return Reduce(delegate, sorter, needMore...)
		},
		"daily": DailyPeriods,
		"multi": MultiSelector,
		"fixed": FixedSelector,
	}
	env = AddArithmetic(env)
	for k, v := range supportedFilters {
		env[k] = v
	}
	environment.apply(env)
	selector, err := mito.Eval(expr, env)
	if err != nil {
		return nil, errs.New("Invalid selector definition '%s', %v", expr, err)
	}
	return selector.(NodeSelectorInit), nil
}

// InvariantFromString parses complex invariants (~declumping rules) from config lines.
func InvariantFromString(expr string) (Invariant, error) {
	if expr == "" {
		return AllGood(), nil
	}
	env := map[any]any{
		"maxcontrol": func(attribute string, max int64) (Invariant, error) {
			attr, err := CreateNodeAttribute(attribute)
			if err != nil {
				return nil, err
			}
			return ClumpingByAttribute(attr, int(max)), nil
		},
		"filter": FilterInvariant,
	}
	for k, v := range supportedFilters {
		env[k] = v
	}
	env[mito.OpAnd] = func(env map[any]any, a, b any) (any, error) {
		filter1, ok1 := a.(NodeFilter)
		filter2, ok2 := b.(NodeFilter)
		if ok1 && ok2 {
			return NodeFilters{filter1, filter2}, nil
		}

		invariant1, ok1 := a.(Invariant)
		invariant2, ok2 := b.(Invariant)
		if ok1 && ok2 {
			return CombinedInvariant(invariant1, invariant2), nil
		}

		return nil, ErrPlacement.New("&& is supported only between NodeFilter or Invariant instances")
	}
	filter, err := mito.Eval(expr, env)
	if err != nil {
		return nil, errs.New("Invalid invariant definition '%s', %v", expr, err)
	}
	return filter.(Invariant), nil
}
