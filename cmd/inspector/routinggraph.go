// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"fmt"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func extendPrefix(prefix string, bit bool) string {
	if bit {
		return prefix + "1"
	}
	return prefix + "0"
}

type bucketID = storj.NodeID

func bufferedGraph(buf *bytes.Buffer, info *pb.GetBucketListResponse) {
	buf.Write([]byte("digraph{\nnode [shape=box];edge [dir=none];\n"))

	buckets := info.GetBuckets()
	addBucketsToGraph(buckets, 0, buf, "")

	buf.Write([]byte("}\n"))

}

func splitBucket(buckets []*pb.GetBucketListResponse_Bucket, bitDepth int) ([]*pb.GetBucketListResponse_Bucket, []*pb.GetBucketListResponse_Bucket) {
	var b0 []*pb.GetBucketListResponse_Bucket
	var b1 []*pb.GetBucketListResponse_Bucket

	for _, bucket := range buckets {
		bID := bucket.BucketId
		byteDepth := bitDepth / 8
		bitOffset := bitDepth % 8
		power := uint(7 - bitOffset)
		bitMask := byte(1 << power)
		b := bID[byteDepth]
		if b&bitMask > 0 {
			b1 = append(b1, bucket)
		} else {
			b0 = append(b0, bucket)
		}
	}
	return b0, b1
}

func printNodeInBuffer(n *pb.Node, buf *bytes.Buffer) {
	fmt.Fprintf(buf, "  %s <i>(%s)</i><br align=\"left\" />", n.Id.String(), n.Address.Address)
}

func addBucketsToGraph(b []*pb.GetBucketListResponse_Bucket, depth int, buf *bytes.Buffer, inPrefix string) {
	if len(b) == 1 {
		addLeafBucketToGraph(b[0], buf, inPrefix)
		return
	}

	b0, b1 := splitBucket(b, depth)

	outPrefix := extendPrefix(inPrefix, false)
	fmt.Fprintf(buf, "b%s [shape=point];", inPrefix)

	addBucketsToGraph(b0, depth+1, buf, outPrefix)
	fmt.Fprintf(buf, "b%s -> b%s [label=<<b>0</b>>];", inPrefix, outPrefix)

	outPrefix = extendPrefix(inPrefix, true)
	addBucketsToGraph(b1, depth+1, buf, outPrefix)
	fmt.Fprintf(buf, "b%s -> b%s [label=<<b>1</b>>];", inPrefix, outPrefix)
}

func addLeafBucketToGraph(b *pb.GetBucketListResponse_Bucket, buf *bytes.Buffer, prefix string) {
	fmt.Fprintf(buf, "b%s [label=<<b><font point-size=\"18\">%s </font></b><br />\n<i>routing:</i><br align=\"left\"/>", prefix, prefix)

	routingNodes := b.GetRoutingNodes()
	for _, n := range routingNodes {
		printNodeInBuffer(n, buf)
	}
	fmt.Fprintf(buf, "<i>cache:</i><br align=\"left\" />")
	cachedNodes := b.GetCachedNodes()
	for _, c := range cachedNodes {
		printNodeInBuffer(c, buf)
	}
	fmt.Fprintf(buf, ">];")
}
