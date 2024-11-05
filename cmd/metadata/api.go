// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"os"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
)

func main() {
	ctx := context.Background()
	log, _ := zap.Config{
		Encoding:    "json",
		Level:       zap.NewAtomicLevelAt(zapcore.DebugLevel),
		OutputPaths: []string{"stdout"},
	}.Build()

	databaseURL := os.Getenv("STORJ_DATABASE")
	db, err := satellitedb.Open(ctx, log.Named("db"), databaseURL, satellitedb.Options{
		ApplicationName: "metadata-api",
	})
	if err != nil {
		log.Error("Error starting master database on metadata api:", zap.Error(err))
		return
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	metabaseURL := os.Getenv("STORJ_METAINFO_DATABASE_URL")
	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), metabaseURL,
		metabase.Config{
			ApplicationName: "metadata-api",
		},
	)
	if err != nil {
		log.Error("Error creating metabase connection on metadata api:", zap.Error(err))
		return
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()
}
