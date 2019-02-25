// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import (
	"go.uber.org/zap"

	"storj.io/storj/pkg/storj"
)

// NodeIDsToLookupRequests converts NodeIDs to LookupRequests
func NodeIDsToLookupRequests(nodeIDs storj.NodeIDList) *LookupRequests {
	var rq []*LookupRequest
	for _, v := range nodeIDs {
		r := &LookupRequest{NodeId: v}
		rq = append(rq, r)
	}
	return &LookupRequests{LookupRequest: rq}
}

// LookupResponsesToNodes converts LookupResponses to Nodes
func LookupResponsesToNodes(responses *LookupResponses) []*Node {
	var nodes []*Node
	for _, v := range responses.LookupResponse {
		n := v.GetNode()
		nodes = append(nodes, n)
	}
	return nodes
}

// NodesToIDs extracts Node-s into a list of ids
func NodesToIDs(nodes []*Node) storj.NodeIDList {
	ids := make(storj.NodeIDList, len(nodes))
	for i, node := range nodes {
		if node != nil {
			ids[i] = node.Id
		}
	}
	return ids
}

// CopyNode returns a deep copy of a node
// It would be better to use `proto.Clone` but it is curently incompatible
// with gogo's customtype extension.
// (see https://github.com/gogo/protobuf/issues/147)
func CopyNode(src *Node) (dst *Node) {
	src.Type.DPanicOnInvalid("copy node")
	node := Node{Id: storj.NodeID{}}
	copy(node.Id[:], src.Id[:])
	if src.Address != nil {
		node.Address = &NodeAddress{
			Transport: src.Address.Transport,
			Address:   src.Address.Address,
		}
	}
	if src.Metadata != nil {
		node.Metadata = &NodeMetadata{
			Email:  src.Metadata.Email,
			Wallet: src.Metadata.Wallet,
		}
	}
	if src.Restrictions != nil {
		node.Restrictions = &NodeRestrictions{
			FreeBandwidth: src.Restrictions.FreeBandwidth,
			FreeDisk:      src.Restrictions.FreeDisk,
		}
	}

	node.Type = src.Type

	return &node
}

// DPanicOnInvalid panics if NodeType is invalid if zap is in development mode,
// otherwise it logs.
func (nt NodeType) DPanicOnInvalid(from string) {
	// TODO: Remove all references
	if nt == NodeType_INVALID {
		zap.L().DPanic("INVALID NODE TYPE: " + from)
	}
}

// AddressEqual compares two node addresses
func AddressEqual(a1, a2 *NodeAddress) bool {
	if a1 == nil && a2 == nil {
		return true
	}
	if a1 == nil || a2 == nil {
		return false
	}
	return a1.Transport == a2.Transport &&
		a1.Address == a2.Address
}
