// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alicebob/miniredis"
	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/datarepair/checker"
	"storj.io/storj/pkg/datarepair/repairer"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/overlay"
	mock "storj.io/storj/pkg/overlay/mocks"
	psserver "storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

const (
	storagenodeCount = 100
)

// Satellite is for configuring client
type Satellite struct {
	Identity    provider.IdentityConfig
	Kademlia    kademlia.Config
	PointerDB   pointerdb.Config
	Overlay     overlay.Config
	Checker     checker.Config
	Repairer    repairer.Config
	MockOverlay struct {
		Enabled bool   `default:"true" help:"if false, use real overlay"`
		Host    string `default:"" help:"if set, the mock overlay will return storage nodes with this host"`
	}
}

// StorageNode is for configuring storage nodes
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
		Satellite    Satellite
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
		address := runCfg.StorageNodes[i].Identity.Address
		if runCfg.Satellite.MockOverlay.Enabled &&
			runCfg.Satellite.MockOverlay.Host != "" {
			_, port, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			address = net.JoinHostPort(runCfg.Satellite.MockOverlay.Host, port)
		}
		storagenode := fmt.Sprintf("%s:%s", identity.ID.String(), address)
		storagenodes = append(storagenodes, storagenode)
		go func(i int, farmer string) {
			_, _ = fmt.Printf("starting farmer %d %s (kad on %s)\n", i, farmer,
				runCfg.StorageNodes[i].Kademlia.TODOListenAddr)
			errch <- runCfg.StorageNodes[i].Identity.Run(ctx, nil,
				runCfg.StorageNodes[i].Kademlia,
				runCfg.StorageNodes[i].Storage)
		}(i, storagenode)
	}

	// start satellite
	go func() {
		_, _ = fmt.Printf("starting satellite on %s\n",
			runCfg.Satellite.Identity.Address)
		var o provider.Responsibility = runCfg.Satellite.Overlay
		if runCfg.Satellite.MockOverlay.Enabled {
			o = mock.Config{Nodes: strings.Join(storagenodes, ",")}
		}

		errch <- runCfg.Satellite.Identity.Run(ctx,
			grpcauth.NewAPIKeyInterceptor(),
			runCfg.Satellite.PointerDB,
			runCfg.Satellite.Kademlia,
			o,
		)
	}()

	// start Repair
	m := miniredis.NewMiniRedis()
	m.RequireAuth("abc123")

	if err = m.StartAddr(":6378"); err != nil {
		errch <- err
	} else {
		defer m.Close()

		go func() {
			errch <- runCfg.Satellite.Checker.Run(ctx, nil)
		}()

		go func() {
			errch <- runCfg.Satellite.Repairer.Run(ctx, nil)
		}()

	}

	// start s3 uplink
	go func() {
		_, _ = fmt.Printf("Starting s3-gateway on %s\nAccess key: %s\nSecret key: %s\n",
			runCfg.Uplink.IdentityConfig.Address, runCfg.Uplink.AccessKey, runCfg.Uplink.SecretKey)
		errch <- runCfg.Uplink.Run(ctx)
	}()

	for v := range errch {
		err = fmt.Errorf("%s : %s", err, v)
	}

	return err
}
