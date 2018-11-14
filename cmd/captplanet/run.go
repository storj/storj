// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/datarepair/checker"
	"storj.io/storj/pkg/datarepair/repairer"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/piecestore/psserver"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/satellite/satelliteweb"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/utils"
)

const (
	storagenodeCount = 100
)

// Satellite is for configuring client
type Satellite struct {
	Identity  provider.IdentityConfig
	Kademlia  kademlia.Config
	PointerDB pointerdb.Config
	Overlay   overlay.Config
	Checker   checker.Config
	Repairer  repairer.Config
	Audit     audit.Config
	StatDB    statdb.Config
	Web       satelliteweb.Config
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
	// start mini redis
	m := miniredis.NewMiniRedis()
	m.RequireAuth("abc123")

	if err = m.StartAddr(":6378"); err != nil {
		errch <- err
	} else {
		defer m.Close()
	}

	// start satellite
	go func() {
		_, _ = fmt.Printf("starting satellite on %s\n",
			runCfg.Satellite.Identity.Address)

		if runCfg.Satellite.Audit.SatelliteAddr == "" {
			runCfg.Satellite.Audit.SatelliteAddr = runCfg.Satellite.Identity.Address
		}

		if runCfg.Satellite.Web.SatelliteAddr == "" {
			runCfg.Satellite.Web.SatelliteAddr = runCfg.Satellite.Identity.Address
		}

		// Run satellite
		errch <- runCfg.Satellite.Identity.Run(ctx,
			grpcauth.NewAPIKeyInterceptor(),
			runCfg.Satellite.PointerDB,
			runCfg.Satellite.Kademlia,
			runCfg.Satellite.Audit,
			runCfg.Satellite.StatDB,
			runCfg.Satellite.Overlay,
			// TODO(coyle): re-enable the checker after we determine why it is panicing
			// runCfg.Satellite.Checker,
			// runCfg.Satellite.Repairer,
			runCfg.Satellite.Web,
		)
	}()

	// start the storagenodes
	for i := 0; i < len(runCfg.StorageNodes); i++ {
		identity, err := runCfg.StorageNodes[i].Identity.Load()
		if err != nil {
			return err
		}
		address := runCfg.StorageNodes[i].Identity.Address
		storagenode := fmt.Sprintf("%s:%s", identity.ID.String(), address)
		go func(i int, storagenode string) {
			_, _ = fmt.Printf("starting storage node %d %s (kad on %s)\n",
				i, storagenode,
				runCfg.StorageNodes[i].Identity.Address)
			errch <- runCfg.StorageNodes[i].Identity.Run(ctx, nil,
				runCfg.StorageNodes[i].Kademlia,
				runCfg.StorageNodes[i].Storage)
		}(i, storagenode)
	}

	// start s3 uplink
	go func() {
		_, _ = fmt.Printf("Starting s3-gateway on %s\nAccess key: %s\nSecret key: %s\n",
			runCfg.Uplink.IdentityConfig.Address, runCfg.Uplink.AccessKey, runCfg.Uplink.SecretKey)
		errch <- runCfg.Uplink.Run(ctx)
	}()

	return utils.CollectErrors(errch, 5*time.Second)
}
