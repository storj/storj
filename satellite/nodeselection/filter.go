// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/storj/location"
)

// NodeFilter can decide if a Node should be part of the selection or not.
type NodeFilter interface {
	Match(node *SelectedNode) bool
}

// NodeFilterWithAnnotation is a NodeFilter with additional annotations.
type NodeFilterWithAnnotation interface {
	NodeFilter
	GetAnnotation(name string) string
}

// Annotation can be used as node filters in 'XX && annotation('...')' like struct.
type Annotation struct {
	Key   string
	Value string
}

// Match implements NodeFilter.
func (a Annotation) Match(node *SelectedNode) bool {
	return true
}

// GetAnnotation implements NodeFilterWithAnnotation.
func (a Annotation) GetAnnotation(name string) string {
	if a.Key == name {
		return a.Value
	}
	return ""
}

func (a Annotation) String() string {
	return fmt.Sprintf(`annotation("%s","%s")`, a.Key, a.Value)
}

var _ NodeFilterWithAnnotation = Annotation{}

// AnnotatedNodeFilter is just a NodeFilter with additional annotations.
type AnnotatedNodeFilter struct {
	Filter      NodeFilter
	Annotations []Annotation
}

// GetAnnotation implements NodeFilterWithAnnotation.
func (a AnnotatedNodeFilter) GetAnnotation(name string) string {
	for _, a := range a.Annotations {
		if a.Key == name {
			return a.Value
		}
	}
	if annotated, ok := a.Filter.(NodeFilterWithAnnotation); ok {
		return annotated.GetAnnotation(name)
	}
	return ""
}

// Match implements NodeFilter.
func (a AnnotatedNodeFilter) Match(node *SelectedNode) bool {
	return a.Filter.Match(node)
}

func (a AnnotatedNodeFilter) String() string {
	var annotationStr []string
	for _, annotation := range a.Annotations {
		annotationStr = append(annotationStr, annotation.String())
	}
	return fmt.Sprintf("%s && %s", a.Filter, strings.Join(annotationStr, " && "))
}

// WithAnnotation adds annotations to a NodeFilter.
func WithAnnotation(filter NodeFilter, name string, value string) NodeFilterWithAnnotation {
	return AnnotatedNodeFilter{
		Filter: filter,
		Annotations: []Annotation{
			{
				Key:   name,
				Value: value,
			},
		},
	}
}

// GetAnnotation retrieves annotation from AnnotatedNodeFilter.
func GetAnnotation(filter NodeFilter, name string) string {
	if annotated, ok := filter.(NodeFilterWithAnnotation); ok {
		return annotated.GetAnnotation(name)
	}
	return ""
}

var _ NodeFilterWithAnnotation = AnnotatedNodeFilter{}

// NodeFilters is a collection of multiple node filters (all should vote with true).
type NodeFilters []NodeFilter

// NodeFilterFunc is helper to use func as NodeFilter.
type NodeFilterFunc func(node *SelectedNode) bool

// Match implements NodeFilter interface.
func (n NodeFilterFunc) Match(node *SelectedNode) bool {
	return n(node)
}

// ExcludeAllFilter will never select any node.
type ExcludeAllFilter struct{}

// Match implements NodeFilter interface.
func (ExcludeAllFilter) Match(node *SelectedNode) bool { return false }

// Match implements NodeFilter interface.
func (n NodeFilters) Match(node *SelectedNode) bool {
	for _, filter := range n {
		if !filter.Match(node) {
			return false
		}
	}
	return true
}

// OrFilter will include the node, if at lest one of the filters are matched.
type OrFilter []NodeFilter

// Match implements NodeFilter interface.
func (n OrFilter) Match(node *SelectedNode) bool {
	for _, filter := range n {
		if filter.Match(node) {
			return true
		}
	}
	return false
}

func (n OrFilter) String() string {
	var parts []string
	for _, filter := range n {
		parts = append(parts, fmt.Sprintf("%s", filter))
	}
	return "(" + strings.Join(parts, " || ") + ")"
}

// WithCountryFilter is a helper to create a new filter with additional CountryFilter.
func (n NodeFilters) WithCountryFilter(permit location.Set) NodeFilters {
	return append(n, NewCountryFilter(permit))
}

// WithExcludedIDs is a helper to create a new filter with additional WithExcludedIDs.
func (n NodeFilters) WithExcludedIDs(ds []storj.NodeID) NodeFilters {
	return append(n, ExcludedIDs(ds))
}

func (n NodeFilters) String() string {
	if len(n) == 1 {
		return fmt.Sprintf("%s", n[0])
	}

	var res []string
	for _, filter := range n {
		res = append(res, fmt.Sprintf("%s", filter))
	}
	sort.Strings(res)
	return "(" + strings.Join(res, " && ") + ")"
}

// GetAnnotation implements NodeFilterWithAnnotation.
func (n NodeFilters) GetAnnotation(name string) string {
	for _, filter := range n {
		if annotated, ok := filter.(NodeFilterWithAnnotation); ok {
			value := annotated.GetAnnotation(name)
			if value != "" {
				return value
			}
		}
	}
	return ""
}

var _ NodeFilterWithAnnotation = NodeFilters{}

// CountryFilter can select nodes based on the condition of the country code.
type CountryFilter struct {
	permit location.Set
}

// NewCountryFilter creates a new CountryFilter.
func NewCountryFilter(permit location.Set) *CountryFilter {
	return &CountryFilter{
		permit: permit,
	}
}

// NewCountryFilterFromString parses country definitions like 'hu','!hu','*','none' and creates a CountryFilter.
func NewCountryFilterFromString(countries []string) (*CountryFilter, error) {
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
			set = apply(set, EuCountries...)
		case "eea":
			set = apply(set, EuCountries...)
			set = apply(set, EeaCountriesWithoutEu...)
		default:
			code := location.ToCountryCode(country)
			if code == location.None {
				return nil, errs.New("invalid country code %q", code)
			}
			set = apply(set, code)
		}
	}
	return NewCountryFilter(set), nil
}

// Match implements NodeFilter interface.
func (p *CountryFilter) Match(node *SelectedNode) bool {
	return p.permit.Contains(node.CountryCode)
}

func (p *CountryFilter) String() string {
	var included, excluded []string
	for country, iso := range location.CountryISOCode {
		if p.permit.Contains(location.CountryCode(country)) {
			included = append(included, iso)
		} else {
			excluded = append(excluded, "!"+iso)
		}
	}
	if len(excluded) < len(included) {
		sort.Strings(excluded)
		return fmt.Sprintf(`country("*","%s")`, strings.Join(excluded, `","`))
	}
	sort.Strings(included)
	return fmt.Sprintf(`country("%s")`, strings.Join(included, `","`))
}

var _ NodeFilter = &CountryFilter{}

// ExcludedNetworks will exclude nodes with specified networks.
type ExcludedNetworks []string

// Match implements NodeFilter interface.
func (e ExcludedNetworks) Match(node *SelectedNode) bool {
	for _, id := range e {
		if id == node.LastNet {
			return false
		}
	}
	return true
}

var _ NodeFilter = ExcludedNetworks{}

// ExcludedNodeNetworks exclude nodes which has same net as the one of the specified.
type ExcludedNodeNetworks []*SelectedNode

// Match implements NodeFilter interface.
func (e ExcludedNodeNetworks) Match(node *SelectedNode) bool {
	for _, n := range e {
		if node.LastNet == n.LastNet {
			return false
		}
	}
	return true
}

var _ NodeFilter = ExcludedNodeNetworks{}

// ExcludedIDs can blacklist NodeIDs.
type ExcludedIDs []storj.NodeID

// Match implements NodeFilter interface.
func (e ExcludedIDs) Match(node *SelectedNode) bool {
	for _, id := range e {
		if id == node.ID {
			return false
		}
	}
	return true
}

var _ NodeFilter = ExcludedIDs{}

// ValueMatch defines how to compare tag value with the defined one.
type ValueMatch func(a []byte, b []byte) bool

// TagFilter matches nodes with specific tags.
type TagFilter struct {
	signer storj.NodeID
	name   string
	value  []byte
	match  ValueMatch
}

// NewTagFilter creates a new tag filter.
func NewTagFilter(id storj.NodeID, name string, value []byte, match ValueMatch) TagFilter {
	return TagFilter{
		signer: id,
		name:   name,
		value:  value,
		match:  match,
	}
}

// Match implements NodeFilter interface.
func (t TagFilter) Match(node *SelectedNode) bool {
	for _, tag := range node.Tags {
		if tag.Name == t.name && t.match(tag.Value, t.value) && tag.Signer == t.signer {
			return true
		}
	}
	return false
}

func (t TagFilter) String() string {
	return fmt.Sprintf(`tag("%s","%s","%s")`, t.signer, t.name, string(t.value))
}

var _ NodeFilter = TagFilter{}

// ExcludeFilter excludes only the matched nodes.
type ExcludeFilter struct {
	matchToExclude NodeFilter
}

// Match implements NodeFilter interface.
func (e ExcludeFilter) Match(node *SelectedNode) bool {
	return !e.matchToExclude.Match(node)
}

func (e ExcludeFilter) String() string {
	return fmt.Sprintf("exclude(%s)", e.matchToExclude)
}

// NewExcludeFilter creates filter, nodes matching the given filter will be excluded.
func NewExcludeFilter(filter NodeFilter) ExcludeFilter {
	return ExcludeFilter{
		matchToExclude: filter,
	}
}

var _ NodeFilter = ExcludeFilter{}

// AnyFilter matches all the nodes.
type AnyFilter struct{}

// Match implements NodeFilter interface.
func (a AnyFilter) Match(node *SelectedNode) bool {
	return true
}

var _ NodeFilter = AnyFilter{}

// AllowedNodesFilter is a special filter which enables only the selected nodes.
type AllowedNodesFilter []storj.NodeID

// AllowedNodesFromFile loads a list of allowed NodeIDs from a text file. One ID per line.
func AllowedNodesFromFile(file string) (AllowedNodesFilter, error) {
	l := AllowedNodesFilter{}
	raw, err := os.ReadFile(file)
	if err != nil {
		return l, errs.Wrap(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		id, err := parseHexOrBase58ID(line)
		if err != nil {
			return l, errs.Wrap(err)
		}
		l = append(l, id)
	}
	return l, nil
}

func parseHexOrBase58ID(line string) (storj.NodeID, error) {
	id, err := storj.NodeIDFromString(line)
	if err == nil {
		return id, nil
	}
	raw, err := hex.DecodeString(line)
	if err != nil {
		return storj.NodeID{}, errs.New("Line is neither hex nor base58 nodeID: %s", line)
	}
	id, err = storj.NodeIDFromBytes(raw)
	if err != nil {
		return storj.NodeID{}, errs.New("Line is neither hex nor base58 nodeID: %s", line)
	}
	return id, nil
}

// Match implements NodeFilter.
func (n AllowedNodesFilter) Match(node *SelectedNode) bool {
	for _, white := range n {
		if node.ID == white {
			return true
		}
	}
	return false
}

var _ NodeFilter = AllowedNodesFilter{}
