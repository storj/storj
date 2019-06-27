// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import (
	"bytes"
	"reflect"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/storj"
)

// Equal compares two Protobuf messages via serialization
func Equal(msg1, msg2 proto.Message) bool {
	//reflect.DeepEqual and proto.Equal don't seem work in all cases
	//todo:  see how slow this is compared to custom equality checks
	if msg1 == nil {
		return msg2 == nil
	}
	if reflect.TypeOf(msg1) != reflect.TypeOf(msg2) {
		return false
	}
	msg1Bytes, err := proto.Marshal(msg1)
	if err != nil {
		return false
	}
	msg2Bytes, err := proto.Marshal(msg2)
	if err != nil {
		return false
	}
	return bytes.Compare(msg1Bytes, msg2Bytes) == 0
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
	node := Node{Id: storj.NodeID{}}
	copy(node.Id[:], src.Id[:])

	if src.Address != nil {
		node.Address = &NodeAddress{
			Transport: src.Address.Transport,
			Address:   src.Address.Address,
		}
	}

	return &node
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
