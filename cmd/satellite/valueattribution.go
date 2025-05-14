// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
)

func cmdPopulatePlacementFromBucketMetainfos(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	batchSize := 1000 // Default batch size.
	if len(args) > 0 {
		parsedBatchSize, err := strconv.Atoi(args[0])
		if err != nil {
			return errs.New("invalid batch size '%s': %v", args[0], err)
		}
		if parsedBatchSize <= 0 {
			return errs.New("batch size must be a positive number")
		}
		batchSize = parsedBatchSize
	}

	satDB, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName:      "satellite-valueattribution",
		APIKeysLRUOptions:    runCfg.APIKeysLRUOptions(),
		RevocationLRUOptions: runCfg.RevocationLRUOptions(),
	})
	if err != nil {
		return errs.New("error connecting to satellite database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, satDB.Close())
	}()

	return populatePlacementFromBucketMetainfos(ctx, log, satDB, batchSize)
}

func populatePlacementFromBucketMetainfos(ctx context.Context, log *zap.Logger, satDB satellite.DB, batchSize int) error {
	log.Info("Starting to populate bucket placement constraints in value attribution",
		zap.Int("batchSize", batchSize))

	var totalRowsProcessed int64
	var batchCount int

	for {
		rowsProcessed, hasMore, err := satDB.Attribution().BackfillPlacementBatch(ctx, batchSize)
		if err != nil {
			return errs.New("error updating placement values: %v", err)
		}

		totalRowsProcessed += rowsProcessed
		batchCount++

		if rowsProcessed > 0 {
			log.Info("Batch completed",
				zap.Int64("rowsUpdated", rowsProcessed),
				zap.Int("batchNumber", batchCount),
				zap.Int64("totalRowsUpdated", totalRowsProcessed))
		}

		// If no more rows to process or the last batch was empty, we're done.
		if !hasMore || rowsProcessed == 0 {
			break
		}
	}

	log.Info("Completed populating bucket placement constraints in value attribution",
		zap.Int64("totalRowsUpdated", totalRowsProcessed),
		zap.Int("batchesRun", batchCount))

	return nil
}
