// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uploadselection

import (
	"storj.io/common/storj"
	"storj.io/common/storj/location"
)

// Node defines necessary information for node-selection.
type Node struct {
	storj.NodeURL
	LastNet     string
	LastIPPort  string
	CountryCode location.CountryCode
}

// Clone returns a deep clone of the selected node.
func (node *Node) Clone() *Node {
	return &Node{
		NodeURL:     node.NodeURL,
		LastNet:     node.LastNet,
		LastIPPort:  node.LastIPPort,
		CountryCode: node.CountryCode,
	}
}
