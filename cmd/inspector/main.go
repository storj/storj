package main

import (
	"context"
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
	addr = "127.0.0.1:7778"
	// ErrInspectorDial throws when there are errors dialing the inspector server
	ErrInspectorDial = errs.Class("error dialing inspector server:")
	rootCmd          = &cobra.Command{
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
	ctx      context.Context
}

// NewInspector creates a new gRPC inspector server for access to kad
// and the overlay cache
func NewInspector(address string) (*Inspector, error) {
	ctx := context.Background()
	identity, err := node.NewFullIdentity(ctx, 12, 4)
	if err != nil {
		return &Inspector{}, err
	}

	tc := transport.NewClient(identity)
	conn, err := tc.DialAddress(ctx, address)
	if err != nil {
		return &Inspector{}, err
	}

	c := pb.NewInspectorClient(conn)

	return &Inspector{
		identity: identity,
		client:   c,
		ctx:      ctx,
	}, nil
}

// CountNodes returns the number of nodes in the cache and kademlia
func CountNodes(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(addr)
	if err != nil {
		return ErrInspectorDial.New("")
	}

	count, err := i.client.CountNodes(i.ctx, &pb.CountNodesRequest{})
	if err != nil {
		errs.New("Could not retrieve node count:")
	}

	fmt.Printf("---------- \n - Kademlia: %+v\n - Overlay: %+v\n", count.Kademlia, count.Overlay)
	return nil
}

// GetBuckets returns all buckets in the overlay cache's routing table
func GetBuckets(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(addr)
	if err != nil {
		return ErrInspectorDial.New("")
	}

	buckets, err := i.client.GetBuckets(i.ctx, &pb.GetBucketsRequest{})
	if err != nil {
		return errs.New("could not retrieve buckets")
	}

	fmt.Printf("Buckets ------------- \n %+v\n", buckets)
	return nil
}

// GetBucket returns a bucket with given `id`
func GetBucket(cmd *cobra.Command, args []string) (err error) {
	if len(args) < 1 {
		return errs.New("Must provide at least one bucket ID")
	}

	fmt.Printf("Looking up bucket %+v\n", args[0])

	i, err := NewInspector(addr)
	if err != nil {
		return ErrInspectorDial.New("")
	}

	bucket, err := i.client.GetBucket(i.ctx, &pb.GetBucketRequest{
		Id: args[0],
	})

	if err != nil {
		return errs.New("could not get bucket")
	}

	fmt.Printf("Bucket ----------- \n %+v\n", bucket)
	return nil
}

func init() {
	rootCmd.AddCommand(countNodeCmd)
	rootCmd.AddCommand(getBucketsCmd)
	rootCmd.AddCommand(getBucketCmd)
}

func main() {
	process.Exec(rootCmd)
}
