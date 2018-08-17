// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"go.uber.org/zap"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
	proto "storj.io/storj/protos/overlay"
)

var (
	mon     = monkit.Package()
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
		Identity provider.IdentityConfig
		Kademlia kademlia.Config
		cacheConfig
	}

	defaultConfDir = "$HOME/.storj/hc"
)

func init() {
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	cfgstruct.Bind(addCmd.Flags(), &cacheCfg)
	cfgstruct.Bind(listCmd.Flags(), &cacheCfg)
}

func cmdList(cmd *cobra.Command, args []string) (err error) {
	f := func(c *overlay.Cache) error {
		keys, err := c.DB.List(nil, 0)
		if err != nil {
			return err
		}

		for _, k := range keys {
			n, err := c.Get(process.Ctx(cmd), string(k))
			if err != nil {
				zap.S().Infof("ID: %s; error getting value\n", k)
			}
			if n != nil {
				zap.S().Infof("ID: %s; Address: %s\n", k, n.Address.Address)
				continue
			}
			zap.S().Infof("ID: %s: nil\n", k)
		}

		return nil
	}
	c := cacheInjector{
		cacheConfig: cacheCfg.cacheConfig,
		c:           f,
	}

	return cacheCfg.Identity.Run(process.Ctx(cmd),
		cacheCfg.Kademlia, c)
}

func cmdAdd(cmd *cobra.Command, args []string) (err error) {
	j, err := ioutil.ReadFile(cacheCfg.NodesPath)
	if err != nil {
		return errs.Wrap(err)
	}

	var nodes map[string]string
	if err := json.Unmarshal(j, &nodes); err != nil {
		return errs.Wrap(err)
	}

	f := func(c *overlay.Cache) error {
		for i, a := range nodes {
			zap.S().Infof("adding node ID: %s; Address: %s", i, a)
			err := c.Put(i, proto.Node{
				Id: i,
				Address: &proto.NodeAddress{
					Transport: 0,
					Address:   a,
				},
				Type: 1,
				// TODO@ASK: Restrictions for staging storage nodes?
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
	c := cacheInjector{
		cacheConfig: cacheCfg.cacheConfig,
		c:           f,
	}

	return cacheCfg.Identity.Run(process.Ctx(cmd),
		cacheCfg.Kademlia, c)
}

func main() {
	addCmd.Flags().String("config",
		filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	listCmd.Flags().String("config",
		filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	process.Exec(rootCmd)
}
