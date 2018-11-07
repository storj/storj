package main

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

var (
	rootCmd = &cobra.Command{
		Use:   "kad",
		Short: "CLI for interacting with Storj Kademlia network",
	}
	lsCmd = &cobra.Command{
		Use:   "ls",
		Short: "List all kad buckets",
		RunE:  ListBuckets,
	}
)

// Client struct for each command to use and get access to the
// cache and kademlia instances
type Client struct {
	kad    *kademlia.Kademlia
	cache  *overlay.Cache
	conn   *grpc.ClientConn
	client pb.KadCliClient
}

// NewClient returns a new *Client struct
func NewClient(cmd *cobra.Command, args []string) (*Client, error) {
	ctx := context.Background()

	ca, err := provider.NewTestCA(ctx)
	if err != nil {
		log.Fatal(err)
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		log.Fatal(err)
	}

	// Set up connection with rpc server
	n := &pb.Node{
		Address: &pb.NodeAddress{
			Address:   ":7777", // default captplanet port
			Transport: 0,
		},
		Id: "kadcli",
	}
	tc := transport.NewClient(identity)
	conn, err := tc.DialNode(ctx, n)
	client := pb.NewKadCliClient(conn)

	return &Client{
		conn:   conn,
		client: client,
	}, nil
}

// ListBuckets lists all kademlia buckets
func ListBuckets(cmd *cobra.Command, args []string) (err error) {
	fmt.Printf("Getting nodes \n")
	ctx := context.Background()
	cli, err := NewClient(cmd, args)
	if err != nil {
		return err
	}

	res, err := cli.client.CountNodes(ctx, &pb.CountNodesRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("Count Nodes: %+v\n", res)

	return err
}

func init() {
	rootCmd.AddCommand(lsCmd)
}

func main() {
	rootCmd.Execute()
}
