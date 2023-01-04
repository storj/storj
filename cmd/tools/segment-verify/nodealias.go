// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"time"

	"storj.io/storj/satellite/metabase"
)

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

// RemoveAll xs from the set.
func (set NodeAliasSet) RemoveAll(xs NodeAliasSet) {
	for x := range xs {
		delete(set, x)
	}
}

type nodeAliasExpiringSet struct {
	nowFunc               func() time.Time
	aliasesAndExpiryTimes map[metabase.NodeAlias]time.Time
	timeToExpire          time.Duration
}

func newNodeAliasExpiringSet(timeToExpire time.Duration) *nodeAliasExpiringSet {
	return &nodeAliasExpiringSet{
		nowFunc:               time.Now,
		aliasesAndExpiryTimes: make(map[metabase.NodeAlias]time.Time),
		timeToExpire:          timeToExpire,
	}
}

// Contains checks whether v was added to the set since the last timeToExpire.
func (expiringSet nodeAliasExpiringSet) Contains(v metabase.NodeAlias) bool {
	expiry, ok := expiringSet.aliasesAndExpiryTimes[v]
	if ok {
		if expiringSet.nowFunc().Before(expiry) {
			return true
		}
		delete(expiringSet.aliasesAndExpiryTimes, v)
	}
	return false
}

// Add adds v to the set.
func (expiringSet nodeAliasExpiringSet) Add(v metabase.NodeAlias) {
	expiringSet.aliasesAndExpiryTimes[v] = expiringSet.nowFunc().Add(expiringSet.timeToExpire)
}

// Remove removes v from the set.
func (expiringSet nodeAliasExpiringSet) Remove(v metabase.NodeAlias) {
	delete(expiringSet.aliasesAndExpiryTimes, v)
}

// AddAll adds all xs to the set.
func (expiringSet nodeAliasExpiringSet) AddAll(xs NodeAliasSet) {
	for x := range xs {
		expiringSet.Add(x)
	}
}
