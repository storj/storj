// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/peertls/tlsopts"
	"storj.io/common/process"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/repaircsv"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/repair/repairer/manual"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/shared/modular"
)

func cmdRepairSegment(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-repair-segment"})
	if err != nil {
		return errs.New("Error starting master database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL,
		runCfg.Config.Metainfo.Metabase("satellite-repair-segment"))
	if err != nil {
		return errs.New("Error creating metabase connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()

	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	config := runCfg

	tlsOptions, err := tlsopts.NewOptions(identity, config.Server.Config, revocationDB)
	if err != nil {
		return err
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)

	placement, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement, nil)
	if err != nil {
		return err
	}

	overlayService, err := overlay.NewService(log.Named("overlay"), db.OverlayCache(), db.NodeEvents(), placement, config.Console.ExternalAddress, config.Console.SatelliteName, config.Overlay)
	if err != nil {
		return err
	}

	orders, err := orders.NewService(
		log.Named("orders"),
		signing.SignerFromFullIdentity(identity),
		overlayService,
		orders.NewNoopDB(),
		placement.CreateFilters,
		config.Orders,
	)
	if err != nil {
		return err
	}

	ecRepairer := repairer.NewECRepairer(
		dialer,
		signing.SigneeFromPeerIdentity(identity.PeerIdentity()),
		config.Repairer.DialTimeout,
		config.Repairer.DownloadTimeout,
		true, true, // force inmemory download and upload of pieces
		config.Repairer.DownloadLongTail)

	segmentRepairer, err := repairer.NewSegmentRepairer(
		log.Named("segment-repair"),
		metabaseDB,
		orders,
		overlayService,
		nil, // TODO add noop version
		ecRepairer,
		placement,
		config.Checker.RepairThresholdOverrides,
		config.Checker.RepairTargetOverrides,
		config.Repairer,
	)
	if err != nil {
		return err
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	group := errgroup.Group{}
	group.Go(func() error {
		return segmentRepairer.Run(cancelCtx)
	})
	group.Go(func() error {
		return overlayService.UploadSelectionCache.Run(cancelCtx)
	})
	defer func() {
		cancel()
		err := group.Wait()
		if err != nil {
			log.Error("upload cache error", zap.Error(err))
		}
	}()
	// Create CSV queue from the first argument (input file path)
	if len(args) == 0 {
		return errs.New("input file path is required as first argument")
	}
	csvQueue, err := repaircsv.NewQueue(repaircsv.Config{InputFile: args[0]}, log.Named("csv"))
	if err != nil {
		return errs.New("failed to create CSV queue: %w", err)
	}
	defer csvQueue.Close()

	mr := manual.NewRepairer(log, metabaseDB, db.OverlayCache(), overlayService, orders, ecRepairer, segmentRepairer,
		csvQueue,
		&modular.StopTrigger{
			Cancel: func() {
				cancel()
			},
		})
	return mr.Run(cancelCtx)
}
