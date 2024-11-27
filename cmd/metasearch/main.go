// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"os"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/metasearch"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
)

func main() {
	ctx := context.Background()
	log, err := zap.NewDevelopment() // TODO: use production logger
	if err != nil {
		panic(err)
	}

	defer log.Sync()

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
	metabase, err := metabase.Open(ctx, log.Named("metabase"), metabaseURL,
		metabase.Config{
			ApplicationName: "metadata-api",
		},
	)
	if err != nil {
		log.Error("Error creating metabase connection on metadata api:", zap.Error(err))
		return
	}
	defer func() {
		err = errs.Combine(err, metabase.Close())
	}()

	endpoint := os.Getenv("STORJ_METASEARCH_ENDPOINT")
	repo := metasearch.NewMetabaseSearchRepository(metabase)
	auth := metasearch.NewHeaderAuth(db)
	metadataAPI, err := metasearch.NewServer(log, repo, auth, endpoint)
	if err != nil {
		log.Error("Error creating metadata api:", zap.Error(err))
		return
	}
	err = metadataAPI.Run()
	if err != nil {
		log.Error("Error running metadata api:", zap.Error(err))
		return
	}
}
