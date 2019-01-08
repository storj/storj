// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"encoding/hex"
	"fmt"
	"io"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Graph ...
func (rt *RoutingTable) Graph(w io.Writer) error {
	_, err := w.Write([]byte("digraph{node [shape=box];"))
	if err != nil {
		return err
	}
	err = rt.graph(w, rt.makeTree())
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("}\n"))
	return err
}

func (rt *RoutingTable) graph(w io.Writer, b *bucket) error {
	if b.split {
		_, err := fmt.Fprintf(w, "b%s [label=\"%s\"];", b.prefix, b.prefix)
		if err != nil {
			return err
		}
		err = rt.graph(w, b.similar)
		if err != nil {
			return err
		}
		err = rt.graph(w, b.dissimilar)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "b%s -> {b%s, b%s};",
			b.prefix, b.similar.prefix, b.dissimilar.prefix)
		return err
	}
	_, err := fmt.Fprintf(w, "b%s [label=\"%s\nrouting:\\l", b.prefix, b.prefix)
	if err != nil {
		return err
	}
	for _, node := range b.nodes {
		_, err = fmt.Fprintf(w, "  %s\\l", hex.EncodeToString(node.Id[:]))
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "cache:\\l")
	if err != nil {
		return err
	}
	for _, node := range b.cache {
		_, err = fmt.Fprintf(w, "  %s\\l", hex.EncodeToString(node.Id[:]))
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "\"];")
	return err
}

func (rt *RoutingTable) makeTree() *bucket {
	var root bucket
	bucketIDs, err := rt.GetBucketIds()
	if err != nil {
		panic("could not get bucketIDs")
	}
	for _, b := range bucketIDs {
		var bID [32]byte
		copy(bID[:], b)
		ns, ok := rt.GetNodes(bID)
		if !ok {
			panic("could not get nodes")
		}
		rc := rt.replacementCache[bID]

		prefix := getPrefix(bID)

		root = bucket{
			prefix: prefix,
			nodes:  ns,
			cache:  rc,
		}
		rt.categorize(&root)
	}
	return &root
}

func getPrefix(bID bucketID) string {
	index, err := determineDifferingBitIndex(firstBucketID, bID)
	if err != nil {
		panic("could not determine differing bit index")
	}
	return string(bID[:index])
}

type bucket struct {
	id     [32]byte
	prefix string
	depth  int

	split      bool
	similar    *bucket
	dissimilar *bucket

	nodes []*pb.Node
	cache []*pb.Node
}

func (rt *RoutingTable) categorize(b *bucket) {
	if b.split {
		if bitAtDepth(b.id, b.depth) == bitAtDepth(rt.self.Id, b.depth) {
			rt.categorize(b.similar)
		} else {
			rt.categorize(b.dissimilar)
		}
		return
	}
	b.split = true
	similarBit := bitAtDepth(rt.self.Id, b.depth)
	b.similar = &bucket{depth: b.depth + 1, prefix: extendPrefix(b.prefix, similarBit)}
	b.dissimilar = &bucket{depth: b.depth + 1, prefix: extendPrefix(b.prefix, !similarBit)}
}

func bitAtDepth(id storj.NodeID, bitDepth int) bool {
	// we could make this a fun one-liner but this is more understandable
	byteDepth := bitDepth / 8
	bitOffset := bitDepth % 8
	power := uint(7 - bitOffset)
	bitMask := byte(1 << power)
	byte_ := id[byteDepth]
	if byte_&bitMask > 0 {
		return true
	}
	return false
}

func extendPrefix(prefix string, bit bool) string {
	if bit {
		return prefix + "1"
	}
	return prefix + "0"
}
