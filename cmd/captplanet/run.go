// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/overlay"
	psserver "storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

const (
	storjnodeCount = 50
)

type HeavyClient struct {
	Identity    provider.IdentityConfig
	Kademlia    kademlia.Config
	PointerDB   pointerdb.Config
	Overlay     overlay.Config
	MockOverlay bool `default:"true" help:"if false, use real overlay"`
}

type StorjNode struct {
	Identity provider.IdentityConfig
	Kademlia kademlia.Config
	Storage  psserver.Config
}

var (
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run all providers",
		RunE:  cmdRun,
	}

	runCfg struct {
		HeavyClient HeavyClient
		StorjNodes  [storjnodeCount]StorjNode
		Gateway     miniogw.Config
	}
)

func init() {
	rootCmd.AddCommand(runCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)
	defer mon.Task()(&ctx)(&err)

	errch := make(chan error, len(runCfg.StorjNodes)+2)
	var storjnodes []string

	// start the storjnodes
	for i := 0; i < len(runCfg.StorjNodes); i++ {
		identity, err := runCfg.StorjNodes[i].Identity.Load()
		if err != nil {
			return err
		}
		storjnode := fmt.Sprintf("%s:%s",
			identity.ID.String(), runCfg.StorjNodes[i].Identity.Address)
		storjnodes = append(storjnodes, storjnode)
		go func(i int, storjnode string) {
			_, _ = fmt.Printf("starting storjnode %d %s (kad on %s)\n", i, storjnode,
				runCfg.StorjNodes[i].Kademlia.TODOListenAddr)
			errch <- runCfg.StorjNodes[i].Identity.Run(ctx,
				runCfg.StorjNodes[i].Kademlia,
				runCfg.StorjNodes[i].Storage)
		}(i, storjnode)
	}

	// start heavy client
	go func() {
		_, _ = fmt.Printf("starting heavy client on %s\n",
			runCfg.HeavyClient.Identity.Address)
		var o provider.Responsibility = runCfg.HeavyClient.Overlay
		if runCfg.HeavyClient.MockOverlay {
			o = overlay.MockConfig{Nodes: strings.Join(storjnodes, ",")}
		}
		errch <- runCfg.HeavyClient.Identity.Run(ctx,
			runCfg.HeavyClient.Kademlia,
			runCfg.HeavyClient.PointerDB,
			o)
	}()

	// start s3 gateway
	go func() {
		_, _ = fmt.Printf("starting minio gateway on %s\n",
			runCfg.Gateway.IdentityConfig.Address)
		errch <- runCfg.Gateway.Run(ctx)
	}()

	return <-errch
}
