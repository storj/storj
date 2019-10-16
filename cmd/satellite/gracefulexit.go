// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/satellitedb"
)

// generateGracefulExitCSV creates a report with graceful exit data for exiting or exited nodes in a given period
func generateGracefulExitCSV(ctx context.Context, start time.Time, end time.Time, output io.Writer) error {
	db, err := satellitedb.New(zap.L().Named("db"), gracefulExitCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	// TODO: add overlay function for getting exiting/exited nodes in timespan
	// TODO: add a config param similar to storagenode/retain config.Status for choosing between all/exiting/exited
	nodeIDs, err := db.OverlayCache().GetGracefulExitNodes(ctx, false, start, end)
	if err != nil {
		return err
	}

	w := csv.NewWriter(output)
	headers := []string{
		"nodeID",
		"walletAddress",
		"nodeCreationDate",
		"initiatedGracefulExit",
		"completedGracefulExit",
		"transferredGB",
	}
	if err := w.Write(headers); err != nil {
		return err
	}

	for _, id := range nodeIDs {
		node, err := db.OverlayCache().Get(ctx, id)
		if err != nil {
			return err
		}
		exitStatus, err := db.OverlayCache().GetExitStatus(ctx, id)
		if err != nil {
			return err
		}

		nextRow := []string{
			node.Id.String(),
			node.Operator.Wallet,
			nil, // TODO how to get creation date
			exitStatus.ExitInitiatedAt.Format("2006-01-02"),
			exitStatus.ExitFinishedAt.Format("2006-01-02"),
			nil, // TODO how to get amount transferred
		}
		if err := w.Write(nextRow); err != nil {
			return err
		}
	}
	if err := w.Error(); err != nil {
		return err
	}
	w.Flush()
	if output != os.Stdout {
		fmt.Println("Generated node usage report for payments")
	}
	return err
}
