// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"storj.io/common/storj"
)

// TrustedOperatorSigner is the zero signer that we use in production. This Go
// syntax is equivalent to
// 0000000000000000000000000000000000000000000000000000000000000100.
var TrustedOperatorSigner = storj.NodeID{30: 1}

// TrustedPeersList represents a configuration-time list of trusted peers.
type TrustedPeersList struct {
	isTrusted map[storj.NodeID]bool
}

// NewTrustedPeerList creates a TrustedPeerList from a list of trusted NodeIDs.
func NewTrustedPeerList(nodes []storj.NodeID) *TrustedPeersList {
	isTrusted := make(map[storj.NodeID]bool, len(nodes))
	for _, node := range nodes {
		isTrusted[node] = true
	}
	return &TrustedPeersList{
		isTrusted: isTrusted,
	}
}

// IsTrusted returns whether a peer is marked as trusted.
func (list *TrustedPeersList) IsTrusted(node storj.NodeID) bool {
	return list.isTrusted[node]
}

// TestingAddTrustedUplink is a helper function for tests to add a trusted uplink.
func (list *TrustedPeersList) TestingAddTrustedUplink(id storj.NodeID) {
	list.isTrusted[id] = true
}
