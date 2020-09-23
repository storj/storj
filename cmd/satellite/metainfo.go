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

	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb"
)

func runMetainfoCmd(cmdFunc func(*metainfo.Service) error) error {
	logger := zap.L()

	db, err := satellitedb.New(logger.Named("db"), runCfg.Database, satellitedb.Options{})
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	pointerDB, err := metainfo.NewStore(logger.Named("pointerdb"), runCfg.Metainfo.DatabaseURL)
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

func fixOldStyleObjects(ctx context.Context, dryRun bool) (err error) {
	return runMetainfoCmd(func(metainfo *metainfo.Service) error {
		var total, fixed int

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			key, err := hex.DecodeString(scanner.Text())
			if err != nil {
				return err
			}

			changed, err := metainfo.FixOldStyleObject(ctx, key, dryRun)
			if err != nil {
				return err
			}

			total++
			if changed {
				fixed++
			}
		}

		if err := scanner.Err(); err != nil {
			return err
		}

		zap.L().Info("Completed.", zap.Int("Fixed", fixed), zap.Int("From Total", total))

		return nil
	})
}
