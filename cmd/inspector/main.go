package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	rootCmd = &cobra.Command{
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
)

// Inspector gives access to kademlia and overlay cache
type Inspector struct {
	overlay  *overlay.Client
	kad      *kademlia.Kademlia
	identity *provider.FullIdentity
	client   *pb.InspectorClient
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

	return &Inspector{}, nil
}

// NewInspector returns an Inspector client
// func NewInspector(address string) (*Inspector, error) {
// 	id, err := node.NewFullIdentity(context.Background(), 12, 4)
// 	if err != nil {
// 		return &Inspector{}, nil
// 	}
// 	overlay, err := overlay.NewOverlayClient(id, address)
// 	if err != nil {
// 		return &Inspector{}, nil
// 	}
//
// 	return &Inspector{
// 		overlay:  overlay,
// 		identity: id,
// 	}, nil
// }

// GetNode returns a node with the requested ID or nothing at all
func GetNode(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector("127.0.0.1:7778")
	if err != nil {
		fmt.Printf("error dialing inspector: %+v\n", err)
		return err
	}

	n := node.IDFromString("testnode")
	found, err := i.overlay.Lookup(context.Background(), n)
	if err != nil {
		return err
	}

	fmt.Printf("### FOUND: %+v\n", found)
	return nil
}

// ListNodes returns the nodes in the cache
func ListNodes(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector("127.0.0.1:7778")
	if err != nil {
		fmt.Printf("error dialing inspector: %+v\n", err)
		return err
	}

	fmt.Printf("Inspector: %+v\n", i)
	return nil
}

func init() {
	rootCmd.AddCommand(getNodeCmd)
	rootCmd.AddCommand(listNodeCmd)
}

func main() {
	process.Exec(rootCmd)
}
