// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"io/ioutil"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
)

var (
	// Error is the error class for overlays
	Error   = errs.Class("overlay error")
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
	c, err := cacheCfg.open()
	if err != nil {
		return err
	}

	keys, err := c.DB.List(nil, 0)
	if err != nil {
		return err
	}
	nodeIDs, err := storj.NodeIDsFromBytes(keys.ByteSlices())
	if err != nil {
		return err
	}

	for _, nodeID := range nodeIDs {
		n, err := c.Get(process.Ctx(cmd), nodeID)
		if err != nil {
			zap.S().Infof("ID: %s; error getting value\n", nodeID.String())
		}
		if n != (storj.Node{}) {
			zap.S().Infof("ID: %s; Address: %s\n", nodeID.String(), n.Address.Address)
			continue
		}
		zap.S().Infof("ID: %s: nil\n", nodeID.String())
	}

	return nil
}

func cmdAdd(cmd *cobra.Command, args []string) (err error) {
	j, err := ioutil.ReadFile(cacheCfg.NodesPath)
	if err != nil {
		return errs.Wrap(err)
	}

	var nodeStrs map[string]string
	if err := json.Unmarshal(j, &nodeStrs); err != nil {
		return errs.Wrap(err)
	}

	c, err := cacheCfg.open()
	if err != nil {
		return err
	}

	var nodeErrs []error
	for nidString, address := range nodeStrs {
		zap.S().Infof("adding node ID: %s; Address: %s", nidString, address)
		nodeID, err := storj.NodeIDFromString(nidString)
		if err != nil {
			nodeErrs = append(nodeErrs, err)
		}

		node := storj.NewNodeWithID(
			nodeID,
			&pb.Node{
				Address: &pb.NodeAddress{
					Transport: 0,
					Address:   address,
				},
				Restrictions: &pb.NodeRestrictions{
					FreeBandwidth: 2000000000,
					FreeDisk:      2000000000,
				},
				Type: 1,
			},
		)
		err = c.Put(node)
		if err != nil {
			return err
		}
	}
	if err = utils.CombineErrors(nodeErrs...); err != nil {
		return err
	}

	return nil
}

func main() {
	process.Exec(rootCmd)
}
