// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"

	"storj.io/common/storj"
)

// TODO: should this file be placed outside of console in nodes package?

// Nodes exposes needed by MND NodesDB functionality.
//
// architecture: Database
type Nodes interface {
	// Add creates new node in NodesDB.
	Add(ctx context.Context, id storj.NodeID, apiSecret []byte, publicAddress string) error
	// GetByID return node from NodesDB by its id.
	GetByID(ctx context.Context, id storj.NodeID) (Node, error)
	// Remove removed node from NodesDB.
	Remove(ctx context.Context, id storj.NodeID) error
}

// Node is a representation of storeganode, that SNO could add to the Multinode Dashboard.
type Node struct {
	ID storj.NodeID
	// APISecret is a secret issued by storagenode, that will be main auth mechanism in MND <-> SNO api. is a secret issued by storagenode, that will be main auth mechanism in MND <-> SNO api.
	APISecret     []byte
	PublicAddress string

	// Logo is a configurable icon.
	Logo []byte
	// Tag is configured by used and could be used to group nodes. // TODO: should node have multiple tags?
	Tag string // TODO: create enum or type in future.
}
