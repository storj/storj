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
	getNodeCmd = &cobra.Command{
		Use:   "get",
		Short: "get node with `id`",
		RunE:  GetNode,
	}
	listNodeCmd = &cobra.Command{
		Use:   "list-nodes",
		Short: "get all nodes in cache",
		RunE:  ListNodes,
	}
	countNodeCmd = &cobra.Command{
		Use:   "count",
		Short: "count nodes in kademlia and overlay",
		RunE:  CountNodes,
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

// GetNode returns a node with the requested ID or nothing at all
func GetNode(cmd *cobra.Command, args []string) (err error) {
	fmt.Printf("Get Node not yet implemented")
	return nil
}

// ListNodes returns the nodes in the cache
func ListNodes(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector("127.0.0.1:7778")
	if err != nil {
		return ErrInspectorDial.New("")
	}

	fmt.Printf("Inspector: %+v\n", i)
	return nil
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

	fmt.Printf("Kademlia: %+v\n Overlay: %+v\n", count.Kademlia, count.Overlay)
	return nil
}

func init() {
	rootCmd.AddCommand(getNodeCmd)
	rootCmd.AddCommand(listNodeCmd)
	rootCmd.AddCommand(countNodeCmd)
}

func main() {
	process.Exec(rootCmd)
}
