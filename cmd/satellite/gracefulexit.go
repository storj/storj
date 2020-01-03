// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
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

func verifyGracefulExitReceipt(ctx context.Context, identity *identity.FullIdentity, nodeID storj.NodeID, receipt string) error {
	signee := signing.SigneeFromPeerIdentity(identity.PeerIdentity())

	bytes, err := hex.DecodeString(receipt)
	if err != nil {
		return errs.Wrap(err)
	}

	// try to unmarshal as an ExitCompleted first
	completed := &pb.ExitCompleted{}
	err = proto.Unmarshal(bytes, completed)
	if err != nil {
		// if it is not a ExitCompleted, try ExitFailed
		failed := &pb.ExitFailed{}
		err = proto.Unmarshal(bytes, failed)
		if err != nil {
			return errs.Wrap(err)
		}

		err = checkIDs(identity.PeerIdentity().ID, nodeID, failed.SatelliteId, failed.NodeId)
		if err != nil {
			return err
		}

		err = signing.VerifyExitFailed(ctx, signee, failed)
		if err != nil {
			return errs.Wrap(err)
		}

		return writeVerificationMessage(false, failed.SatelliteId, failed.NodeId, failed.Failed)
	}

	err = checkIDs(identity.PeerIdentity().ID, nodeID, completed.SatelliteId, completed.NodeId)
	if err != nil {
		return err
	}

	err = signing.VerifyExitCompleted(ctx, signee, completed)
	if err != nil {
		return errs.Wrap(err)
	}

	return writeVerificationMessage(true, completed.SatelliteId, completed.NodeId, completed.Completed)
}

func checkIDs(satelliteID storj.NodeID, providedSNID storj.NodeID, receiptSatelliteID storj.NodeID, receiptSNID storj.NodeID) error {
	if satelliteID != receiptSatelliteID {
		return errs.New("satellite ID (%v) does not match receipt satellite ID (%v).", satelliteID, receiptSatelliteID)
	}
	if providedSNID != receiptSNID {
		return errs.New("provided storage node ID (%v) does not match receipt node ID (%v).", providedSNID, receiptSNID)
	}
	return nil
}

func writeVerificationMessage(succeeded bool, satelliteID storj.NodeID, snID storj.NodeID, timestamp time.Time) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Succeeded:\t%v\n", succeeded)
	fmt.Fprintf(w, "Satellite ID:\t%v\n", satelliteID)
	fmt.Fprintf(w, "Storage Node ID:\t%v\n", snID)
	fmt.Fprintf(w, "Timestamp:\t%v\n", timestamp)

	return errs.Wrap(w.Flush())
}
