// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func randID() (rv storj.NodeID) {
	_, err := rand.Read(rv[:])
	if err != nil {
		panic(err)
	}
	return rv
}

func ID(hexID string) (rv storj.NodeID) {
	bytes, err := hex.DecodeString(hexID)
	if err != nil {
		panic(err)
	}
	if len(bytes) != len(storj.NodeID{}) {
		panic(fmt.Sprintf("invalid length for %q", hexID))
	}
	copy(rv[:], bytes)
	return rv
}

func PadID(hexPrefix, hexPad string) storj.NodeID {
	repeats := (len(storj.NodeID{})*2 - len(hexPrefix)) / len(hexPad)
	return ID(hexPrefix + strings.Repeat(hexPad, repeats))
}

func Node(id storj.NodeID, address string) *pb.Node {
	return &pb.Node{
		Id: id,
		Address: &pb.NodeAddress{
			Address: address,
		},
	}
}

func requireNodesEqual(t testing.TB, expected []*pb.Node, actual []*pb.Node) {
	require.Equal(t, len(expected), len(actual))
	for i, node := range expected {
		require.Equal(t, node.Id, actual[i].Id)
		require.Equal(t, node.Address.Transport, actual[i].Address.Transport)
		require.Equal(t, node.Address.Address, actual[i].Address.Address)
	}
}

func NodeFromPrefix(prefix string, pad string) *pb.Node {
	return Node(PadID(prefix, pad), fmt.Sprintf("address-%s:1", prefix))
}
