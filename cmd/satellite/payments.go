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

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/satellite/satellitedb"
)

// generateCSV generates a payment report for all nodes for a given period
func generateCSV(ctx context.Context, start time.Time, end time.Time, output io.Writer) error {
	db, err := satellitedb.New(paymentsCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	rows, err := db.Accounting().QueryPaymentInfo(ctx, start, end)
	if err != nil {
		return err
	}

	w := csv.NewWriter(output)
	headers := []string{
		"nodeID",
		"nodeCreationDate",
		"auditSuccessRatio",
		"byte-hours:AtRest",
		"bytes:BWRepair-GET",
		"bytes:BWRepair-PUT",
		"bytes:BWAudit",
		"bytes:BWGet",
		"bytes:BWPut",
		"date",
		"walletAddress",
	}
	if err := w.Write(headers); err != nil {
		return err
	}

	for _, row := range rows {
		nid := row.NodeID
		wallet, err := db.OverlayCache().GetWalletAddress(ctx, nid)
		if err != nil {
			return err
		}
		row.Wallet = wallet
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
		fmt.Println("Generated payment report")
	}
	return err
}

func structToStringSlice(s *accounting.CSVRow) []string {
	record := []string{
		s.NodeID.String(),
		s.NodeCreationDate.Format("2006-01-02"),
		strconv.FormatFloat(s.AuditSuccessRatio, 'f', 5, 64),
		strconv.FormatFloat(s.AtRestTotal, 'f', 5, 64),
		strconv.FormatInt(s.GetRepairTotal, 10),
		strconv.FormatInt(s.PutRepairTotal, 10),
		strconv.FormatInt(s.GetAuditTotal, 10),
		strconv.FormatInt(s.GetTotal, 10),
		strconv.FormatInt(s.PutTotal, 10),
		s.Date.Format("2006-01-02"),
		s.Wallet,
	}
	return record
}
