// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"fmt"

	"storj.io/storj/pkg/pb"
)


// FetchLocalGraph returns the routing table as a dot graph
func (rt *RoutingTable) BuildDotGraph() []byte {
	var buf bytes.Buffer
	rt.BufferedGraph(&buf)
	return buf.Bytes()
}


func splitBucket(bIDs []bucketID, bitDepth int) ([]bucketID, []bucketID) {
	var b0 []bucketID
	var b1 []bucketID

	for _, bID := range bIDs {
		byteDepth := bitDepth / 8
		bitOffset := bitDepth % 8
		power := uint(7 - bitOffset)
		bitMask := byte(1 << power)
		b := bID[byteDepth]
		if b&bitMask > 0 {
			b1 = append(b1, bID)
		} else {
			b0 = append(b0, bID)
		}
	}
	return b0, b1
}

// BufferedGraph prints the routing table graph as a dot graph in the specified buffer
func (rt *RoutingTable) BufferedGraph(buf *bytes.Buffer) {
	buf.Write([]byte("digraph{\nnode [shape=box];edge [dir=none];\n"))

	ids, _ := rt.GetBucketIds()
	var bucketids []bucketID
	for _, n := range ids {
		bucketids = append(bucketids, keyToBucketID(n))
	}

	rt.addBucketsToGraph(bucketids, 0, buf, "")

	buf.Write([]byte("}\n"))

}

func (rt *RoutingTable) addLeafBucketToGraph(b bucketID, buf *bytes.Buffer, prefix string) {
	fmt.Fprintf(buf, "b%s [label=<<b><font point-size=\"18\">%s </font></b><br />\n<i>routing:</i><br align=\"left\"/>", prefix, prefix)

	nodes, _ := rt.getUnmarshaledNodesFromBucket(b)
	for _, n := range nodes {
		printNodeInBuffer(n, buf)
	}
	fmt.Fprintf(buf, "<i>cache:</i><br align=\"left\" />")
	cachedNodes, _ := rt.replacementCache[b]
	for _, c := range cachedNodes {
		printNodeInBuffer(c, buf)
	}
	fmt.Fprintf(buf, ">];")
}

func printNodeInBuffer(n *pb.Node, buf *bytes.Buffer) {
	fmt.Fprintf(buf, "  %s <i>(%s)</i><br align=\"left\" />", n.Id.String(), n.Address.Address)
}

func (rt *RoutingTable) addBucketsToGraph(b []bucketID, depth int, buf *bytes.Buffer, inPrefix string) {
	if len(b) == 1 {
		rt.addLeafBucketToGraph(b[0], buf, inPrefix)
		return
	}

	b0, b1 := splitBucket(b, depth)

	outPrefix := extendPrefix(inPrefix, false)
	fmt.Fprintf(buf, "b%s [shape=point];", inPrefix)

	rt.addBucketsToGraph(b0, depth+1, buf, outPrefix)
	fmt.Fprintf(buf, "b%s -> b%s [label=<<b>0</b>>];", inPrefix, outPrefix)

	outPrefix = extendPrefix(inPrefix, true)
	rt.addBucketsToGraph(b1, depth+1, buf, outPrefix)
	fmt.Fprintf(buf, "b%s -> b%s [label=<<b>1</b>>];", inPrefix, outPrefix)
}
