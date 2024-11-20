// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

// AuditedNodes is an interface for filtering nodes for audit.
type AuditedNodes interface {
	// Reload supposed to initialize state. Called before audit ranged-loop cycle.
	Reload(ctx context.Context) error

	// Match should decide if node is part of the selection or not.
	Match(alias metabase.NodeAlias) bool
}

// AllowNodes will use only nodes that match the filter.
type AllowNodes struct {
	filter     nodeselection.NodeFilter
	db         overlay.DB
	metabaseDB *metabase.DB
	whiteList  []bool
}

// NewFilteredNodes creates a new AllowNodes.
func NewFilteredNodes(filter nodeselection.NodeFilter, db overlay.DB, metabaseDB *metabase.DB) *AllowNodes {
	return &AllowNodes{
		filter:     filter,
		db:         db,
		metabaseDB: metabaseDB,
	}
}

// Reload implements AuditedNodes interface.
func (f *AllowNodes) Reload(ctx context.Context) error {
	nodes, err := f.db.GetAllParticipatingNodes(ctx, -12*time.Hour, 0)
	if err != nil {
		return err
	}

	aliasMap, err := f.metabaseDB.LatestNodesAliasMap(ctx)
	if err != nil {
		return err
	}

	maxAlias := aliasMap.Max()
	if maxAlias == -1 {
		return nil
	}

	f.whiteList = make([]bool, maxAlias+1)

	for _, node := range nodes {
		if f.filter.Match(&node) {
			alias, found := aliasMap.Alias(node.ID)
			if found {
				f.whiteList[alias] = true
			}
		}
	}
	return nil
}

// Match implements AuditedNodes interface.
func (f *AllowNodes) Match(alias metabase.NodeAlias) bool {
	return f.whiteList[alias]
}

// AllNodes is a AuditedNodes that includes all nodes.
type AllNodes struct{}

// Reload implements AuditedNodes interface.
func (a AllNodes) Reload(ctx context.Context) error {
	return nil
}

// Match implements AuditedNodes interface.
func (a AllNodes) Match(alias metabase.NodeAlias) bool {
	return true
}

var _ AuditedNodes = (*AllNodes)(nil)
