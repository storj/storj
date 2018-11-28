// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
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

	lookupCfg struct {
		overlay.LookupConfig
	}
)

// Inspector gives access to kademlia and overlay cache
type Inspector struct {
	overlay  overlay.Client
	identity *provider.FullIdentity
}

// NewInspector returns an Inspector client
func NewInspector(address string) (*Inspector, error) {
	id, err := provider.NewFullIdentity(context.Background(), 12, 4)
	if err != nil {
		return &Inspector{}, nil
	}
	overlay, err := overlay.NewOverlayClient(id, address)
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


	ids, err := lookupCfg.ParseIDs()
	if err != nil {
		return err
	}

	var (
		nodes []*pb.Node
		lookupErrs []error
	)
	for _, id := range ids {
		node, err := i.overlay.Lookup(process.Ctx(cmd), id)
		if err != nil {
			lookupErrs = append(lookupErrs, err)
			continue
		}
		nodes = append(nodes, node)
	}
	p := func() {
		for _, n := range nodes {
			fmt.Printf("### FOUND: %+v\n", n)
		}
	}
	if err := utils.CombineErrors(lookupErrs...); err != nil {
		zap.S().Errorf("lookup error(s):", err)
		p()
		return err
	}

	p()
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

