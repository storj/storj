// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"bytes"

	"storj.io/common/storj"
	"storj.io/common/storj/location"
)

// NodeFilter can decide if a Node should be part of the selection or not.
type NodeFilter interface {
	MatchInclude(node *SelectedNode) bool
}

// AnnotatedNodeFilter is just a NodeFilter with additional annotations.
type AnnotatedNodeFilter struct {
	Filter      NodeFilter
	Annotations map[string]string
}

// MatchInclude implements NodeFilter.
func (a AnnotatedNodeFilter) MatchInclude(node *SelectedNode) bool {
	return a.Filter.MatchInclude(node)
}

// WithAnnotation adds annotations to a NodeFilter.
func WithAnnotation(filter NodeFilter, name string, value string) NodeFilter {
	if anf, ok := filter.(AnnotatedNodeFilter); ok {
		anf.Annotations[name] = value
		return anf
	}
	return AnnotatedNodeFilter{
		Filter: filter,
		Annotations: map[string]string{
			name: value,
		},
	}
}

// GetAnnotation retrieves annotation from AnnotatedNodeFilter.
func GetAnnotation(filter NodeFilter, name string) string {
	if annotated, ok := filter.(AnnotatedNodeFilter); ok {
		return annotated.Annotations[name]
	}
	return ""
}

var _ NodeFilter = AnnotatedNodeFilter{}

// NodeFilters is a collection of multiple node filters (all should vote with true).
type NodeFilters []NodeFilter

// NodeFilterFunc is helper to use func as NodeFilter.
type NodeFilterFunc func(node *SelectedNode) bool

// MatchInclude implements NodeFilter interface.
func (n NodeFilterFunc) MatchInclude(node *SelectedNode) bool {
	return n(node)
}

// ExcludeAllFilter will never select any node.
type ExcludeAllFilter struct{}

// MatchInclude implements NodeFilter interface.
func (ExcludeAllFilter) MatchInclude(node *SelectedNode) bool { return false }

// MatchInclude implements NodeFilter interface.
func (n NodeFilters) MatchInclude(node *SelectedNode) bool {
	for _, filter := range n {
		if !filter.MatchInclude(node) {
			return false
		}
	}
	return true
}

// WithCountryFilter is a helper to create a new filter with additional CountryFilter.
func (n NodeFilters) WithCountryFilter(permit location.Set) NodeFilters {
	return append(n, NewCountryFilter(permit))
}

// WithAutoExcludeSubnets is a helper to create a new filter with additional AutoExcludeSubnets.
func (n NodeFilters) WithAutoExcludeSubnets() NodeFilters {
	return append(n, NewAutoExcludeSubnets())
}

// WithExcludedIDs is a helper to create a new filter with additional WithExcludedIDs.
func (n NodeFilters) WithExcludedIDs(ds []storj.NodeID) NodeFilters {
	return append(n, ExcludedIDs(ds))
}

var _ NodeFilter = NodeFilters{}

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

// MatchInclude implements NodeFilter interface.
func (p *CountryFilter) MatchInclude(node *SelectedNode) bool {
	return p.permit.Contains(node.CountryCode)
}

var _ NodeFilter = &CountryFilter{}

// AutoExcludeSubnets pick at most one node from network.
//
// Stateful!!! should be re-created for each new selection request.
// It should only be used as the last filter.
type AutoExcludeSubnets struct {
	seenSubnets map[string]struct{}
}

// NewAutoExcludeSubnets creates an initialized AutoExcludeSubnets.
func NewAutoExcludeSubnets() *AutoExcludeSubnets {
	return &AutoExcludeSubnets{
		seenSubnets: map[string]struct{}{},
	}
}

// MatchInclude implements NodeFilter interface.
func (a *AutoExcludeSubnets) MatchInclude(node *SelectedNode) bool {
	if _, found := a.seenSubnets[node.LastNet]; found {
		return false
	}
	a.seenSubnets[node.LastNet] = struct{}{}
	return true
}

var _ NodeFilter = &AutoExcludeSubnets{}

// ExcludedNetworks will exclude nodes with specified networks.
type ExcludedNetworks []string

// MatchInclude implements NodeFilter interface.
func (e ExcludedNetworks) MatchInclude(node *SelectedNode) bool {
	for _, id := range e {
		if id == node.LastNet {
			return false
		}
	}
	return true
}

var _ NodeFilter = ExcludedNetworks{}

// ExcludedIDs can blacklist NodeIDs.
type ExcludedIDs []storj.NodeID

// MatchInclude implements NodeFilter interface.
func (e ExcludedIDs) MatchInclude(node *SelectedNode) bool {
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

// MatchInclude implements NodeFilter interface.
func (t TagFilter) MatchInclude(node *SelectedNode) bool {
	for _, tag := range node.Tags {
		if tag.Name == t.name && bytes.Equal(tag.Value, t.value) && tag.Signer == t.signer {
			return true
		}
	}
	return false
}

var _ NodeFilter = TagFilter{}
