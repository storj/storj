// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode

import (
	"storj.io/storj/pkg/node"
)

// These methods are added to have same interface as in testplanet to make transition easier.

// NewNodeClient creates a node client for this node
// TODO: this is temporary and only intended for tests
func (peer *Peer) NewNodeClient() (node.Client, error) {
	// TODO: handle disconnect verification
	return node.NewNodeClient(peer.Identity, peer.Local(), peer.Kademlia.Service)
}
