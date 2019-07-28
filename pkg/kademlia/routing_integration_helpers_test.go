// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func id(hexID string) (rv storj.NodeID) {
	bytes, err := hex.DecodeString(hexID)
	if err != nil {
		panic(err)
	}
	if len(bytes) != len(storj.NodeID{}) {
		panic(fmt.Sprintf("invalid length for %q", hexID))
	}
	copy(rv[:], bytes)
	if rv == (storj.NodeID{}) {
		panic("to allow routing table implementations to use a node id zero value (unlikely to have a collision), tests shouldn't use it")
	}
	return rv
}

func PadID(hexPrefix, hexPad string) storj.NodeID {
	repeats := (len(storj.NodeID{})*2 - len(hexPrefix)) / len(hexPad)
	return id(hexPrefix + strings.Repeat(hexPad, repeats))
}

func Node(id storj.NodeID, address string) *pb.Node {
	return &pb.Node{
		Id: id,
		Address: &pb.NodeAddress{
			Address: address,
		},
	}
}

var graphCounter = new(int64)

type Grapher interface {
	Graph(io.Writer) error
}

func SaveGraph(table interface{}) {
	if table, ok := table.(Grapher); ok {
		fh, err := os.Create(fmt.Sprintf("routing-graph-%003d.dot", atomic.AddInt64(graphCounter, 1)))
		if err != nil {
			panic(err)
		}
		defer func() {
			err := fh.Close()
			if err != nil {
				panic(err)
			}
		}()
		err = table.Graph(fh)
		if err != nil {
			panic(err)
		}
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
