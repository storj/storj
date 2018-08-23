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
	farmerCount = 50
)

type HeavyClient struct {
	Identity    provider.IdentityConfig
	Kademlia    kademlia.Config
	PointerDB   pointerdb.Config
	Overlay     overlay.Config
	MockOverlay bool `default:"true" help:"if false, use real overlay"`
}

type Farmer struct {
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
		Farmers     [farmerCount]Farmer
		Uplink     miniogw.Config
	}
)

func init() {
	rootCmd.AddCommand(runCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)
	defer mon.Task()(&ctx)(&err)

	errch := make(chan error, len(runCfg.Farmers)+2)
	var farmers []string

	// start the farmers
	for i := 0; i < len(runCfg.Farmers); i++ {
		identity, err := runCfg.Farmers[i].Identity.Load()
		if err != nil {
			return err
		}
		farmer := fmt.Sprintf("%s:%s",
			identity.ID.String(), runCfg.Farmers[i].Identity.Address)
		farmers = append(farmers, farmer)
		go func(i int, farmer string) {
			_, _ = fmt.Printf("starting farmer %d %s (kad on %s)\n", i, farmer,
				runCfg.Farmers[i].Kademlia.TODOListenAddr)
			errch <- runCfg.Farmers[i].Identity.Run(ctx,
				runCfg.Farmers[i].Kademlia,
				runCfg.Farmers[i].Storage)
		}(i, farmer)
	}

	// start heavy client
	go func() {
		_, _ = fmt.Printf("starting heavy client on %s\n",
			runCfg.HeavyClient.Identity.Address)
		var o provider.Responsibility = runCfg.HeavyClient.Overlay
		if runCfg.HeavyClient.MockOverlay {
			o = overlay.MockConfig{Nodes: strings.Join(farmers, ",")}
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
