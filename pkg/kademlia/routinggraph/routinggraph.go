// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package routinggraph

import (
	"bytes"
	"fmt"
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
)

func extendPrefix(prefix string, bit bool) string {
	if bit {
		return prefix + "1"
	}
	return prefix + "0"
}

// Draw writes the routing graph obtained using a GetBucketListResponse in the specified file
func Draw(file *os.File, info *pb.GetBucketListResponse) (err error) {
	_, err = file.Write([]byte("digraph{\nnode [shape=plaintext, fontname=\"Courier\"];edge [dir=none];rankdir=LR;\n"))
	if err != nil {
		return err
	}
	defer func() {
		_, errWrite := file.Write([]byte("}\n"))
		err = errs.Combine(err, errWrite)
		err = errs.Combine(err, file.Close())
	}()
	var buf bytes.Buffer
	buckets := info.GetBuckets()
	addBucketsToGraph(buckets, 0, &buf, "")
	_, err = buf.WriteTo(os.Stdout)
	return err
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

func printNodeInBuffer(n *pb.Node, buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<TR><TD ALIGN=\"LEFT\">%s</TD><TD SIDES=\"R\" ALIGN=\"LEFT\">%s</TD></TR>", n.Id.String(), n.Address.Address)
}

func addLeafBucketToGraph(b *pb.GetBucketListResponse_Bucket, buf *bytes.Buffer, prefix string) {
	fmt.Fprintf(buf,
		"b%s [label=< <TABLE CELLBORDER=\"0\"><TR><TD CELLSPACING=\"0\" SIDES=\"B\" BORDER=\"1\" COLSPAN=\"2\"><B> %s </B></TD></TR>", prefix, prefix)
	defer fmt.Fprintf(buf, "</TABLE>>];")

	fmt.Fprintf(buf, "<TR><TD  COLSPAN=\"2\" ALIGN=\"LEFT\"><I><B>Routing:</B></I></TD></TR>")
	routingNodes := b.GetRoutingNodes()
	for _, n := range routingNodes {
		printNodeInBuffer(n, buf)
	}
	fmt.Fprintf(buf, "<TR><TD  COLSPAN=\"2\"></TD></TR>")
	fmt.Fprintf(buf, "<TR><TD  COLSPAN=\"2\" ALIGN=\"LEFT\"><I><B>Cache:</B></I></TD></TR>")
	cachedNodes := b.GetCachedNodes()
	for _, c := range cachedNodes {
		printNodeInBuffer(c, buf)
	}
}
