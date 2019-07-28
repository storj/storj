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

	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/satellitedb"
)

// generateCSV creates a report with node usage data for all nodes in a given period which can be used for payments
func generateCSV(ctx context.Context, start time.Time, end time.Time, output io.Writer) error {
	db, err := satellitedb.New(zap.L().Named("db"), nodeUsageCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	rows, err := db.StoragenodeAccounting().QueryPaymentInfo(ctx, start, end)
	if err != nil {
		return err
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

	for _, row := range rows {
		nid := row.NodeID

		node, err := db.OverlayCache().Get(ctx, nid)
		if err != nil {
			return err
		}

		row.Wallet = node.Operator.Wallet
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

func structToStringSlice(s *accounting.CSVRow) []string {
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
