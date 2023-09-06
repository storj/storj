// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

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

// WithCountryFilter is a helper to create a new filter with additional CountryFilter.
func (n NodeFilters) WithCountryFilter(permit location.Set) NodeFilters {
	return append(n, NewCountryFilter(permit))
}

// WithExcludedIDs is a helper to create a new filter with additional WithExcludedIDs.
func (n NodeFilters) WithExcludedIDs(ds []storj.NodeID) NodeFilters {
	return append(n, ExcludedIDs(ds))
}

func (n NodeFilters) String() string {
	var res []string
	for _, filter := range n {
		res = append(res, fmt.Sprintf("%s", filter))
	}
	sort.Strings(res)
	return strings.Join(res, " && ")
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
func NewCountryFilter(permit location.Set) NodeFilter {
	return &CountryFilter{
		permit: permit,
	}
}

// Match implements NodeFilter interface.
func (p *CountryFilter) Match(node *SelectedNode) bool {
	return p.permit.Contains(node.CountryCode)
}

func (p *CountryFilter) String() string {
	var included, excluded []string
	for country, iso := range location.CountryISOCode {
		if p.permit.Contains(country) {
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

// TagFilter matches nodes with specific tags.
type TagFilter struct {
	signer storj.NodeID
	name   string
	value  []byte
}

// NewTagFilter creates a new tag filter.
func NewTagFilter(id storj.NodeID, name string, value []byte) TagFilter {
	return TagFilter{
		signer: id,
		name:   name,
		value:  value,
	}
}

// Match implements NodeFilter interface.
func (t TagFilter) Match(node *SelectedNode) bool {
	for _, tag := range node.Tags {
		if tag.Name == t.name && bytes.Equal(tag.Value, t.value) && tag.Signer == t.signer {
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
