package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/overlay"
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
)

// Inspector gives access to kademlia and overlay cache
type Inspector struct {
	overlay  overlay.Client
	identity *provider.FullIdentity
}

func identity() (*provider.FullIdentity, error) {
	ca, err := provider.NewTestCA(context.Background())
	if err != nil {
		zap.S().Errorf("Failed to create certificate authority: ", zap.Error(err))
		os.Exit(1)
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		zap.S().Errorf("Failed to create full identity: ", zap.Error(err))
		os.Exit(1)
	}
	return identity, nil
}

// overlay returns an overlay client
func newOverlayClient(identity *provider.FullIdentity, address string) (overlay.Client, error) {
	return overlay.NewOverlayClient(identity, address)
}

// NewInspector returns an Inspector client
func NewInspector(address string) (*Inspector, error) {
	id, err := identity()
	overlay, err := newOverlayClient(id, address)
	if err != nil {
		return &Inspector{}, nil
	}

	return &Inspector{
		overlay:  overlay,
		identity: id,
	}, nil
}

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

func init() {
	rootCmd.AddCommand(getNodeCmd)
}

func main() {
	rootCmd.Execute()
}
