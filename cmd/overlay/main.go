// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
)

var (
	rootCmd = &cobra.Command{
		Use:   "overlay",
		Short: "Overlay cache management",
	}
	addCmd = &cobra.Command{
		Use:   "add",
		Short: "Add nodes to the overlay cache",
		RunE:  cmdAdd,
	}
	listCmd = &cobra.Command{
		Use:   "list",
		Short: "List nodes in the overlay cache",
		RunE:  cmdList,
	}

	cacheCfg struct {
		cacheConfig
	}
)

func init() {
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	cfgstruct.Bind(addCmd.Flags(), &cacheCfg)
	cfgstruct.Bind(listCmd.Flags(), &cacheCfg)
}

func cmdList(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)
	cache, dbClose, err := cacheCfg.open(ctx)
	if err != nil {
		return err
	}
	defer dbClose()

	keys, err := cache.Inspect(ctx)
	if err != nil {
		return err
	}

	nodeIDs, err := storj.NodeIDsFromBytes(keys.ByteSlices())
	if err != nil {
		return err
	}

	const padding = 3
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.Debug)
	fmt.Fprintln(w, "Node ID\t Address")

	for _, id := range nodeIDs {
		n, err := cache.Get(process.Ctx(cmd), id)
		if err != nil {
			fmt.Fprintln(w, id.String(), "\t", "error getting value")
		}
		if n != nil {
			fmt.Fprintln(w, id.String(), "\t", n.Address.Address)
			continue
		}
		fmt.Fprintln(w, id.String(), "\tnil")
	}

	return w.Flush()
}

func cmdAdd(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)
	j, err := ioutil.ReadFile(cacheCfg.NodesPath)
	if err != nil {
		return errs.New("Unable to read file with nodes: %+v", err)
	}

	var nodes map[string]string
	if err := json.Unmarshal(j, &nodes); err != nil {
		return errs.Wrap(err)
	}

	cache, dbClose, err := cacheCfg.open(ctx)
	if err != nil {
		return err
	}
	defer dbClose()

	for i, a := range nodes {
		id, err := storj.NodeIDFromString(i)
		if err != nil {
			zap.S().Error(err)
		}
		fmt.Printf("adding node ID: %s; Address: %s", i, a)
		err = cache.Put(process.Ctx(cmd), id, pb.Node{
			Id: id,
			Address: &pb.NodeAddress{
				Transport: 0,
				Address:   a,
			},
			Restrictions: &pb.NodeRestrictions{
				FreeBandwidth: 2000000000,
				FreeDisk:      2000000000,
			},
			Type: pb.NodeType_STORAGE,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	process.Exec(rootCmd)
}
