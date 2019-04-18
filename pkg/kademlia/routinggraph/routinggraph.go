// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package routinggraph

import (
	"fmt"
	"io"

	"storj.io/storj/pkg/pb"
)

type dot struct {
	out io.Writer
	err error
}

func (dot *dot) printf(format string, args ...interface{}) {
	if dot.err != nil {
		return
	}
	_, dot.err = fmt.Fprintf(dot.out, format, args...)
}

// Draw writes the routing graph obtained using a GetBucketListResponse in the specified file
func Draw(w io.Writer, info *pb.GetBucketListResponse) (err error) {
	dot := dot{out: w}
	dot.printf("digraph{\nnode [shape=plaintext, fontname=\"Courier\"];edge [dir=none];\n")
	defer dot.printf("}\n")

	buckets := info.GetBuckets()
	dot.addBuckets(buckets, 0, "")
	return dot.err
}

func (dot *dot) addBuckets(b []*pb.GetBucketListResponse_Bucket, depth int, inPrefix string) {
	if len(b) == 1 {
		dot.Leaf(b[0], inPrefix)
		return
	}

	left, right := splitBucket(b, depth)

	outPrefix := extendPrefix(inPrefix, false)
	dot.printf("b%s [shape=point];", inPrefix)

	dot.addBuckets(left, depth+1, outPrefix)
	dot.Edge(inPrefix, outPrefix, "0")

	outPrefix = extendPrefix(inPrefix, true)
	dot.addBuckets(right, depth+1, outPrefix)
	dot.Edge(inPrefix, outPrefix, "1")
}

func (dot *dot) Edge(inPrefix, outPrefix, label string) {
	dot.printf("b%s -> b%s [label=<<b>%s</b>>];", inPrefix, outPrefix, label)
}

func (dot *dot) Leaf(b *pb.GetBucketListResponse_Bucket, prefix string) {
	dot.printf("b%s [label=< <table cellborder=\"0\"><tr><td cellspacing=\"0\" sides=\"b\" border=\"1\" colspan=\"2\"><b> %s </b></td></tr>", prefix, prefix)
	defer dot.printf("</table>>];")

	dot.printf("<tr><td  colspan=\"2\" align=\"left\"><i><b>routing:</b></i></td></tr>")
	routingNodes := b.GetRoutingNodes()
	for _, n := range routingNodes {
		dot.Node(n)
	}
	dot.printf("<tr><td  colspan=\"2\"></td></tr>")
	dot.printf("<tr><td  colspan=\"2\" align=\"left\"><i><b>cache:</b></i></td></tr>")
	cachedNodes := b.GetCachedNodes()
	for _, c := range cachedNodes {
		dot.Node(c)
	}
}

func (dot *dot) Node(node *pb.Node) {
	dot.printf(`<tr><td align="left">%s</td><td sides="r" align="left">%s</td></tr>`, node.Id, node.Address.Address)
}

func splitBucket(buckets []*pb.GetBucketListResponse_Bucket, bitDepth int) (left, right []*pb.GetBucketListResponse_Bucket) {
	for _, bucket := range buckets {
		bID := bucket.BucketId
		byteDepth := bitDepth / 8
		bitOffset := bitDepth % 8
		power := uint(7 - bitOffset)
		bitMask := byte(1 << power)
		b := bID[byteDepth]
		if b&bitMask > 0 {
			right = append(right, bucket)
		} else {
			left = append(left, bucket)
		}
	}
	return
}

func extendPrefix(prefix string, bit bool) string {
	if bit {
		return prefix + "1"
	}
	return prefix + "0"
}
