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
	storagenodeCount = 50
)

type HeavyClient struct {
	Identity    provider.IdentityConfig
	Kademlia    kademlia.Config
	PointerDB   pointerdb.Config
	Overlay     overlay.Config
	MockOverlay bool `default:"true" help:"if false, use real overlay"`
}

type StorageNode struct {
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
		HeavyClient  HeavyClient
		StorageNodes [storagenodeCount]StorageNode
		Uplink       miniogw.Config
	}
)

func init() {
	rootCmd.AddCommand(runCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)
	defer mon.Task()(&ctx)(&err)

	errch := make(chan error, len(runCfg.StorageNodes)+2)
	var storagenodes []string

	// start the storagenodes
	for i := 0; i < len(runCfg.StorageNodes); i++ {
		identity, err := runCfg.StorageNodes[i].Identity.Load()
		if err != nil {
			return err
		}
		storagenode := fmt.Sprintf("%s:%s",
			identity.ID.String(), runCfg.StorageNodes[i].Identity.Address)
		storagenodes = append(storagenodes, storagenode)
		go func(i int, storagenode string) {
			_, _ = fmt.Printf("starting storagenode %d %s (kad on %s)\n", i, storagenode,
				runCfg.StorageNodes[i].Kademlia.TODOListenAddr)
			errch <- runCfg.StorageNodes[i].Identity.Run(ctx,
				runCfg.StorageNodes[i].Kademlia,
				runCfg.StorageNodes[i].Storage)
		}(i, storagenode)
	}

	// start heavy client
	go func() {
		_, _ = fmt.Printf("starting heavy client on %s\n",
			runCfg.HeavyClient.Identity.Address)
		var o provider.Responsibility = runCfg.HeavyClient.Overlay
		if runCfg.HeavyClient.MockOverlay {
			o = overlay.MockConfig{Nodes: strings.Join(storagenodes, ",")}
		}
		errch <- runCfg.HeavyClient.Identity.Run(ctx,
			runCfg.HeavyClient.Kademlia,
			runCfg.HeavyClient.PointerDB,
			o)
	}()

	// start s3 uplink
	go func() {
		_, _ = fmt.Printf("starting minio uplink on %s\n",
			runCfg.Uplink.IdentityConfig.Address)
		errch <- runCfg.Uplink.Run(ctx)
	}()

	return <-errch
}
