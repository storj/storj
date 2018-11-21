// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

var (
	// Addr is the address of Capt Planet from command flags
	Addr = flag.String("address", "[::1]:7778", "address of captplanet to inspect")

	// ErrInspectorDial throws when there are errors dialing the inspector server
	ErrInspectorDial = errs.Class("error dialing inspector server:")

	// ErrRequest is for gRPC request errors after dialing
	ErrRequest = errs.Class("error processing request:")

	// ErrIdentity is for errors during identity creation for this CLI
	ErrIdentity = errs.Class("error creating identity:")

	// Commander CLI
	rootCmd = &cobra.Command{
		Use:   "inspector",
		Short: "CLI for interacting with Storj Kademlia network",
	}
	countNodeCmd = &cobra.Command{
		Use:   "count",
		Short: "count nodes in kademlia and overlay",
		RunE:  CountNodes,
	}
	getBucketsCmd = &cobra.Command{
		Use:   "list-buckets",
		Short: "get all buckets in overlay",
		RunE:  GetBuckets,
	}
	getBucketCmd = &cobra.Command{
		Use:   "ls <bucket_id>",
		Short: "get all nodes in bucket",
		RunE:  GetBucket,
	}
)

// Inspector gives access to kademlia and overlay cache
type Inspector struct {
	identity *provider.FullIdentity
	client   pb.InspectorClient
}

// NewInspector creates a new gRPC inspector server for access to kad
// and the overlay cache
func NewInspector(address string) (*Inspector, error) {
	ctx := context.Background()
	identity, err := node.NewFullIdentity(ctx, 12, 4)
	if err != nil {
		return &Inspector{}, ErrIdentity.Wrap(err)
	}

	tc := transport.NewClient(identity)
	conn, err := tc.DialAddress(ctx, address)
	if err != nil {
		return &Inspector{}, ErrInspectorDial.Wrap(err)
	}

	c := pb.NewInspectorClient(conn)

	return &Inspector{
		identity: identity,
		client:   c,
	}, nil
}

// CountNodes returns the number of nodes in the cache and kademlia
func CountNodes(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	count, err := i.client.CountNodes(context.Background(), &pb.CountNodesRequest{})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Printf("---------- \n - Kademlia: %+v\n - Overlay: %+v\n", count.Kademlia, count.Overlay)
	return nil
}

// GetBuckets returns all buckets in the overlay cache's routing table
func GetBuckets(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	buckets, err := i.client.GetBuckets(context.Background(), &pb.GetBucketsRequest{})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Printf("Buckets ---------------- \n Total Buckets: %+v\n", buckets.Total)

	for index, b := range buckets.Ids {
		fmt.Printf("%+v %+v\n", index, b)
	}
	return nil
}

// GetBucket returns a bucket with given `id`
func GetBucket(cmd *cobra.Command, args []string) (err error) {
	if len(args) < 1 {
		return errs.New("Must provide at least one bucket ID")
	}

	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	bucket, err := i.client.GetBucket(context.Background(), &pb.GetBucketRequest{
		Id: args[0],
	})

	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Printf("Bucket ----------- \n %+v\n", bucket)
	return nil
}

func init() {
	rootCmd.AddCommand(countNodeCmd)
	rootCmd.AddCommand(getBucketsCmd)
	rootCmd.AddCommand(getBucketCmd)
	flag.Parse()
}

func main() {
	process.Exec(rootCmd)
}
