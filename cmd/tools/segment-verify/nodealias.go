// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import "storj.io/storj/satellite/metabase"

// NodeAliasSet is a set containing node aliases.
type NodeAliasSet map[metabase.NodeAlias]struct{}

// Contains checks whether v is in the set.
func (set NodeAliasSet) Contains(v metabase.NodeAlias) bool {
	_, ok := set[v]
	return ok
}

// Add v to the set.
func (set NodeAliasSet) Add(v metabase.NodeAlias) {
	set[v] = struct{}{}
}

// Remove v from the set.
func (set NodeAliasSet) Remove(v metabase.NodeAlias) {
	delete(set, v)
}
