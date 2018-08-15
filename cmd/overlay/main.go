// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/overlay"
	proto "storj.io/storj/protos/overlay"
)

type Config struct {
	NodesPath   string `help:"the path to a JSON file containing an object with IP keys and nodeID values" default:"$CONFDIR/nodes.json"`
	DatabaseURL string `help:"the database connection string to use" default:"bolt://$CONFDIR/overlay.db"`
}

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
	clearCmd = &cobra.Command{
		Use:   "clear",
		Short: "Clear the overlay cache",
		RunE:  cmdClear,
	}

	addCfg struct {
		Identity provider.IdentityConfig
		Kademlia kademlia.Config
		Overlay  Config
	}

	clearCfg struct {
		ExceptPath string
	}

	defaultConfDir = "$HOME/.storj/hc"
)

func init() {
	rootCmd.AddCommand(addCmd)
	cfgstruct.Bind(addCmd.Flags(), &addCfg)
	cfgstruct.Bind(clearCmd.Flags(), &clearCfg)
}

func cmdAdd(cmd *cobra.Command, args []string) (err error) {
	return addCfg.Identity.Run(process.Ctx(cmd), addCfg.Kademlia, addCfg.Overlay)
}

func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	j, err := ioutil.ReadFile(addCfg.Overlay.NodesPath)
	if err != nil {
		return errs.Wrap(err)
	}

	var nodes map[string]string
	if err := json.Unmarshal(j, &nodes); err != nil {
		return errs.Wrap(err)
	}

	kad := kademlia.LoadFromContext(ctx)
	if kad == nil {
		return Error.New("programmer error: kademlia responsibility unstarted")
	}

	dburl, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return Error.Wrap(err)
	}

	var cache *overlay.Cache
	switch dburl.Scheme {
	case "bolt":
		cache, err = overlay.NewBoltOverlayCache(dburl.Path, kad)
		if err != nil {
			return err
		}
		zap.S().Info("Starting overlay cache with BoltDB")
	case "redis":
		db, err := strconv.Atoi(dburl.Query().Get("db"))
		if err != nil {
			return Error.New("invalid db: %s", err)
		}
		cache, err = overlay.NewRedisOverlayCache(dburl.Host, overlay.UrlPwd(dburl), db, kad)
		if err != nil {
			return err
		}
		zap.S().Info("Starting overlay cache with Redis")
	default:
		return Error.New("database scheme not supported: %s", dburl.Scheme)
	}

	fmt.Println(nodes)
	for i, a := range nodes {
		cache.Put(i, proto.Node{
			Address: &proto.NodeAddress{
				Transport: 0,
				Address: a,
			},
			Type: 1,
			// TODO@ASK: Restrictions for staging storage nodes?
		})
	}

	return nil
}

func cmdClear(cmd *cobra.Command, args []string) (err error) {
	// TODO
	return nil
}

func main() {
	addCmd.Flags().String("config",
		filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	process.Exec(rootCmd)
}
