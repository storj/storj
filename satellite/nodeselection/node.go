// Copyright (C) 2020 Storj Labs, Incache.
// See LICENSE for copying information.

package nodeselection

import (
	"storj.io/common/pb"
	"storj.io/common/storj"
)

// Node defines necessary information for node-selection.
type Node struct {
	ID         storj.NodeID
	Address    *pb.NodeAddress
	LastNet    string
	LastIPPort string
}

// Clone returns a deep clone of the selected node.
func (node *Node) Clone() *Node {
	return &Node{
		ID: node.ID,
		Address: &pb.NodeAddress{
			Transport: node.Address.Transport,
			Address:   node.Address.Address,
		},
		LastNet:    node.LastNet,
		LastIPPort: node.LastIPPort,
	}
}
