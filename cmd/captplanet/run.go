// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/overlay"
	psserver "storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
	proto "storj.io/storj/protos/overlay"
)

const (
	farmerCount = 50
)

type HeavyClient struct {
	Identity  provider.IdentityConfig
	Kademlia  kademlia.Config
	PointerDB pointerdb.Config
	Overlay   overlay.Config
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

	errch := make(chan error, len(runCfg.Farmers)+2)

	// start heavy client
	go func() {
		_, _ = fmt.Printf("starting heavy client on %s\n",
			runCfg.HeavyClient.Identity.Address)
		errch <- runCfg.HeavyClient.Identity.Run(ctx,
			runCfg.HeavyClient.Kademlia,
			runCfg.HeavyClient.PointerDB,
			runCfg.HeavyClient.Overlay)
	}()

	// start the farmers
	for i := 0; i < len(runCfg.Farmers); i++ {
		go func(i int) {
			_, _ = fmt.Printf("starting farmer %d grpc on %s, kad on %s\n", i,
				runCfg.Farmers[i].Identity.Address,
				runCfg.Farmers[i].Kademlia.TODOListenAddr)
			errch <- runCfg.Farmers[i].Identity.Run(ctx,
				runCfg.Farmers[i].Kademlia,
				runCfg.Farmers[i].Storage)
		}(i)
		identity, err := runCfg.Farmers[i].Identity.Load()
		if err != nil {
			return err
		}
		miniogw.GlobalMockOverlay.Nodes[identity.ID.String()] = &proto.Node{
			Id: identity.ID.String(),
			Address: &proto.NodeAddress{
				Transport: proto.NodeTransport_TCP,
				Address:   runCfg.Farmers[i].Identity.Address,
			},
		}
	}

	// start s3 gateway
	go func() {
		_, _ = fmt.Printf("starting minio gateway on %s\n",
			runCfg.Gateway.IdentityConfig.Address)
		errch <- runCfg.Gateway.Run(ctx)
	}()

	return <-errch
}
