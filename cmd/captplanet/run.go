// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/satellite/satellitedb"
)

const (
	storagenodeCount = 10
)

var (
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run all servers",
		RunE:  cmdRun,
	}

	runCfg Captplanet
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

	satellite := runCfg.Satellite
	// start satellite
	go func() {
		_, _ = fmt.Printf("Starting satellite on %s\n", satellite.Server.Address)

		if satellite.Audit.SatelliteAddr == "" {
			satellite.Audit.SatelliteAddr = satellite.Server.Address
		}

		if satellite.Web.SatelliteAddr == "" {
			satellite.Web.SatelliteAddr = satellite.Server.Address
		}

		database, err := satellitedb.New(satellite.Database)
		if err != nil {
			errch <- errs.New("Error starting master database on satellite: %+v", err)
			return
		}

		err = database.CreateTables()
		if err != nil {
			errch <- errs.New("Error creating tables for master database on satellite: %+v", err)
			return
		}

		//nolint ignoring context rules to not create cyclic dependency, will be removed later
		satelliteCtx := context.WithValue(ctx, "masterdb", database)

		// Run satellite
		errch <- satellite.Server.Run(satelliteCtx,
			grpcauth.NewAPIKeyInterceptor(),
			satellite.Kademlia,
			satellite.Overlay,
			satellite.Discovery,
			satellite.PointerDB,
			satellite.Audit,
			satellite.Checker,
			satellite.Repairer,
			satellite.BwAgreement,
			satellite.Web,
			satellite.Tally,
			satellite.Rollup,
			satellite.StatDB,
		)
	}()

	// hack-fix t oensure that satellite gets up and running before starting storage nodes
	time.Sleep(2 * time.Second)

	// start the storagenodes
	for i, v := range runCfg.StorageNodes {
		go func(i int, v StorageNode) {
			identity, err := v.Server.Identity.Load()
			if err != nil {
				return
			}

			address := v.Server.Address
			storagenode := fmt.Sprintf("%s:%s", identity.ID.String(), address)

			_, _ = fmt.Printf("Starting storage node %d %s (kad on %s)\n", i, storagenode, address)
			errch <- v.Server.Run(ctx, nil, v.Kademlia, v.Storage)
		}(i, v)
	}

	// start s3 uplink
	uplink := runCfg.Uplink
	go func() {
		_, _ = fmt.Printf("Starting s3-gateway on %s\nAccess key: %s\nSecret key: %s\n",
			uplink.Server.Address,
			uplink.Minio.AccessKey,
			uplink.Minio.SecretKey)
		errch <- uplink.Run(ctx)
	}()

	return utils.CollectErrors(errch, 5*time.Second)
}
