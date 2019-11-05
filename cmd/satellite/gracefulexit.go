// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/satellitedb"
)

// generateGracefulExitCSV creates a report with graceful exit data for exiting or exited nodes in a given period
func generateGracefulExitCSV(ctx context.Context, completed bool, start time.Time, end time.Time, output io.Writer) error {
	db, err := satellitedb.New(zap.L().Named("db"), gracefulExitCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	var nodeIDs storj.NodeIDList
	if completed {
		nodeIDs, err = db.OverlayCache().GetGracefulExitCompletedByTimeFrame(ctx, start, end)
		if err != nil {
			return err
		}
	} else {
		nodeIDs, err = db.OverlayCache().GetGracefulExitIncompleteByTimeFrame(ctx, start, end)
		if err != nil {
			return err
		}
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
		exitProgress, err := db.GracefulExit().GetProgress(ctx, id)
		if gracefulexit.ErrNodeNotFound.Has(err) {
			exitProgress = &gracefulexit.Progress{}
		} else if err != nil {
			return err
		}

		exitStatus := node.ExitStatus
		exitFinished := ""
		if exitStatus.ExitFinishedAt != nil {
			exitFinished = exitStatus.ExitFinishedAt.Format("2006-01-02")
		}
		nextRow := []string{
			node.Id.String(),
			node.Operator.Wallet,
			node.CreatedAt.Format("2006-01-02"),
			exitStatus.ExitInitiatedAt.Format("2006-01-02"),
			exitFinished,
			strconv.FormatInt(exitProgress.BytesTransferred, 10),
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
		if completed {
			fmt.Println("Generated report for nodes that have completed graceful exit.")
		} else {
			fmt.Println("Generated report for nodes that are in the process of graceful exiting.")
		}
	}
	return err
}
