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

	"storj.io/common/storj"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb"
)

// UsageCSVRow represents data from NodePaymentInfo without exposing dbx.
type UsageCSVRow struct {
	accounting.NodePaymentInfo
	overlay.NodeAccountingInfo
}

// generateNodeUsageCSV creates a report with node usage data for all nodes in a given period which can be used for payments.
func generateNodeUsageCSV(ctx context.Context, start time.Time, end time.Time, output io.Writer) error {
	db, err := satellitedb.Open(ctx, zap.L().Named("db"), nodeUsageCfg.Database, satellitedb.Options{ApplicationName: "satellite-nodeusage"})
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	paymentInfos, err := db.StoragenodeAccounting().QueryPaymentInfo(ctx, start, end)
	if err != nil {
		return err
	}

	var nodeIDs storj.NodeIDList
	for _, paymentInfo := range paymentInfos {
		nodeIDs = append(nodeIDs, paymentInfo.NodeID)
	}

	nodes, err := db.OverlayCache().AccountingNodeInfo(ctx, nodeIDs)
	if err != nil {
		return err
	}

	rows := make([]UsageCSVRow, len(paymentInfos))
	for i, paymentInfo := range paymentInfos {
		node, ok := nodes[paymentInfo.NodeID]
		if !ok {
			return errs.New("node info missing for %v", paymentInfo.NodeID)
		}
		rows[i] = UsageCSVRow{
			NodePaymentInfo:    paymentInfo,
			NodeAccountingInfo: node,
		}
	}

	w := csv.NewWriter(output)
	headers := []string{
		"nodeID",
		"nodeCreationDate",
		"byte-hours:AtRest",
		"bytes:BWRepair-GET",
		"bytes:BWRepair-PUT",
		"bytes:BWAudit",
		"bytes:BWPut",
		"bytes:BWGet",
		"walletAddress",
		"disqualified",
	}
	if err := w.Write(headers); err != nil {
		return err
	}

	for i := range rows {
		row := &rows[i]
		record := structToStringSlice(row)
		if err := w.Write(record); err != nil {
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

func structToStringSlice(s *UsageCSVRow) []string {
	dqStr := ""
	if s.Disqualified != nil {
		dqStr = s.Disqualified.Format("2006-01-02")
	}
	record := []string{
		s.NodeID.String(),
		s.NodeCreationDate.Format("2006-01-02"),
		strconv.FormatFloat(s.AtRestTotal, 'f', 5, 64),
		strconv.FormatInt(s.GetRepairTotal, 10),
		strconv.FormatInt(s.PutRepairTotal, 10),
		strconv.FormatInt(s.GetAuditTotal, 10),
		strconv.FormatInt(s.PutTotal, 10),
		strconv.FormatInt(s.GetTotal, 10),
		s.Wallet,
		dqStr,
	}
	return record
}
