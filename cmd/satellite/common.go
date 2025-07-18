// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
)

func checkDBVersions(ctx context.Context, log *zap.Logger, cfg Satellite, satelliteDB satellite.DB, metabaseDB *metabase.DB) error {
	err := metabaseDB.CheckVersion(ctx)
	if err != nil {
		message := "Failed metabase database version check."
		if !cfg.UnsafeSkipDBVersionCheck {
			log.Log(zap.ErrorLevel, message, zap.Error(err))
			return errs.New("failed metabase version check: %+v", err)
		}
		log.Log(zap.WarnLevel, message, zap.Error(err))
	}

	err = satelliteDB.CheckVersion(ctx)
	if err != nil {
		message := "Failed satellite database version check."
		if !cfg.UnsafeSkipDBVersionCheck {
			log.Log(zap.ErrorLevel, message, zap.Error(err))
			return errs.New("failed satellite version check: %+v", err)
		}
		log.Log(zap.WarnLevel, message, zap.Error(err))
	}
	return nil
}
