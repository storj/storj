// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"os"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb"
)

func runMetainfoCmd(ctx context.Context, cmdFunc func(*metainfo.Service) error) error {
	logger := zap.L()

	db, err := satellitedb.New(logger.Named("db"), dryRunCfg.Database, satellitedb.Options{})
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	err = db.CheckVersion(ctx)
	if err != nil {
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	pointerDB, err := metainfo.NewStore(logger.Named("pointerdb"), dryRunCfg.Metainfo.DatabaseURL)
	if err != nil {
		return errs.New("Error creating metainfo database connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, pointerDB.Close())
	}()

	service := metainfo.NewService(
		logger.Named("metainfo:service"),
		pointerDB,
		db.Buckets(),
	)

	return cmdFunc(service)
}

func runVerifierCmd(ctx context.Context, cmdFunc func(*audit.Verifier) error) error {
	log := zap.L()

	identity, err := dryRunCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	db, err := satellitedb.New(log.Named("db"), dryRunCfg.Database, satellitedb.Options{})
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	err = db.CheckVersion(ctx)
	if err != nil {
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	pointerDB, err := metainfo.NewStore(log.Named("pointerdb"), dryRunCfg.Metainfo.DatabaseURL)
	if err != nil {
		return errs.New("Error creating metainfo database connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, pointerDB.Close())
	}()

	revocationDB, err := revocation.NewDBFromCfg(dryRunCfg.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	tlsOptions, err := tlsopts.NewOptions(identity, dryRunCfg.Server.Config, revocationDB)
	if err != nil {
		return errs.New("Error creating TLS options: %+v", err)
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)

	metainfoService := metainfo.NewService(
		log.Named("metainfo:service"),
		pointerDB,
		db.Buckets(),
	)

	overlayService := overlay.NewService(
		log.Named("overlay"),
		db.OverlayCache(),
		runCfg.Overlay,
	)

	ordersService, err := orders.NewService(
		log.Named("orders:service"),
		signing.SignerFromFullIdentity(identity),
		overlayService,
		db.Orders(),
		db.Buckets(),
		runCfg.Orders,
		&pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   runCfg.Contact.ExternalAddress,
		},
	)
	if err != nil {
		return errs.New("Error creating orders service: %+v", err)
	}

	verifier := audit.NewVerifier(
		log.Named("audit:verifier"),
		metainfoService,
		dialer,
		overlayService,
		db.Containment(),
		ordersService,
		identity,
		runCfg.Audit.MinBytesPerSecond,
		runCfg.Audit.MinDownloadTimeout,
	)

	return cmdFunc(verifier)
}

func fixOldStyleObjects(ctx context.Context) (err error) {
	return runMetainfoCmd(ctx, func(metainfo *metainfo.Service) error {
		var total, fixed int

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			key, err := hex.DecodeString(scanner.Text())
			if err != nil {
				return err
			}

			changed, err := metainfo.FixOldStyleObject(ctx, key, dryRunCfg.DryRun)
			if err != nil {
				return err
			}

			total++
			if changed {
				fixed++
			}
		}

		zap.L().Info("Completed.", zap.Int("Fixed", fixed), zap.Int("From Total", total))

		return scanner.Err()
	})
}

func verifyPieceHashes(ctx context.Context) (err error) {
	return runVerifierCmd(ctx, func(verifier *audit.Verifier) error {
		var total, fixed int

		verifier.UsedToVerifyPieceHashes = true

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			key, err := hex.DecodeString(scanner.Text())
			if err != nil {
				return err
			}

			changed, err := verifier.VerifyPieceHashes(ctx, string(key), dryRunCfg.DryRun)
			if err != nil {
				return err
			}

			total++
			if changed {
				fixed++
			}
		}

		zap.L().Info("Completed.", zap.Int("Fixed", fixed), zap.Int("From Total", total))

		return scanner.Err()
	})
}
