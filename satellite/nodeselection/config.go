// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"bytes"
	"os"
	"strings"

	"github.com/jtolio/mito"
	"github.com/zeebo/errs"
	"gopkg.in/yaml.v3"

	"storj.io/common/storj"
)

// placementConfig is the representation of YAML based placement configuration.
type placementConfig struct {

	// helpers which can be re-used later to simplify config
	Templates map[string]string

	// the placement definitions
	Placements []placementDefinition
}

type placementDefinition struct {
	ID        storj.PlacementConstraint
	Name      string
	Filter    string
	Invariant string
	Selector  string
}

// LoadConfig loads the placement yaml file and creates the Placement definitions.
func LoadConfig(configFile string) (PlacementDefinitions, error) {
	placements := make(PlacementDefinitions)

	cfg := &placementConfig{}
	raw, err := os.ReadFile(configFile)
	if err != nil {
		return placements, errs.New("Couldn't load placement config from file %s: %v", configFile, err)
	}
	err = yaml.Unmarshal(raw, &cfg)
	if err != nil {
		return placements, errs.New("Couldn't parse placement config as YAML from file  %s: %v", configFile, err)
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
		}

		filter := resolveTemplates(def.Filter)
		p.NodeFilter, err = filterFromString(filter)
		if err != nil {
			return placements, errs.New("Filter definition '%s' of placement %d is invalid: %v", filter, def.ID, err)
		}

		invariant := resolveTemplates(def.Invariant)
		p.Invariant, err = invariantFromString(invariant)
		if err != nil {
			return placements, errs.New("Invariant definition '%s' of placement %d is invalid: %v", invariant, def.ID, err)
		}

		selector := resolveTemplates(def.Selector)
		p.Selector, err = selectorFromString(selector)
		if err != nil {
			return placements, errs.New("Selector definition '%s' of placement %d is invalid: %v", selector, def.ID, err)
		}

		placements[def.ID] = p
	}
	return placements, nil
}

var supportedFilters = map[any]any{
	"country": func(countries ...string) (NodeFilter, error) {
		return NewCountryFilterFromString(countries)
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
}

func filterFromString(expr string) (NodeFilter, error) {
	if expr == "" {
		expr = "all()"
	}
	filter, err := mito.Eval(expr, supportedFilters)
	if err != nil {
		return nil, errs.New("Invalid filter definition '%s', %v", expr, err)
	}
	return filter.(NodeFilter), nil
}

func selectorFromString(expr string) (NodeSelectorInit, error) {
	if expr == "" {
		expr = "random()"
	}
	env := map[any]any{
		"attribute": func(attribute string) (NodeSelectorInit, error) {
			attr, err := CreateNodeAttribute(attribute)
			if err != nil {
				return nil, err
			}
			return AttributeGroupSelector(attr), nil
		},
		"random": func() (NodeSelectorInit, error) {
			return RandomSelector(), nil
		},
		"unvetted": func(newNodeRatio float64, def NodeSelectorInit) (NodeSelectorInit, error) {
			return UnvettedSelector(newNodeRatio, def), nil
		},
		"nodelist": AllowedNodesFromFile,
		"filter":   FilterSelector,
		"balanced": func(attribute string) (NodeSelectorInit, error) {
			attr, err := CreateNodeAttribute(attribute)
			if err != nil {
				return nil, err
			}
			return BalancedGroupBasedSelector(attr), nil
		},
	}
	for k, v := range supportedFilters {
		env[k] = v
	}
	selector, err := mito.Eval(expr, env)
	if err != nil {
		return nil, errs.New("Invalid selector definition '%s', %v", expr, err)
	}
	return selector.(NodeSelectorInit), nil
}

func invariantFromString(expr string) (Invariant, error) {
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
	}
	filter, err := mito.Eval(expr, env)
	if err != nil {
		return nil, errs.New("Invalid invariant definition '%s', %v", expr, err)
	}
	return filter.(Invariant), nil
}
