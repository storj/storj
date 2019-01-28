// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/satellite/satellitedb"
)

// generateCSV generates a payment report for all nodes for a given period
func generateCSV(ctx context.Context, cfgDir string, dbPath string, id string, start time.Time, end time.Time) (string, error) {

	db, err := satellitedb.New(dbPath)
	if err != nil {
		return "", errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			fmt.Printf("error closing connection to master database on satellite: %+v\n", err)
		}
	}()

	pDir := filepath.Join(cfgDir, "payments")

	if err := os.MkdirAll(pDir, 0700); err != nil {
		return "", err
	}

	layout := "2006-01-02"
	filename := id + "--" + start.Format(layout) + "--" + end.Format(layout) + ".csv"
	path := filepath.Join(pDir, filename)
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer utils.LogClose(file)

	rows, err := db.Accounting().QueryPaymentInfo(ctx, start, end)
	if err != nil {
		return "", err
	}

	w := csv.NewWriter(file)
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
		return "", err
	}

	for _, row := range rows {
		nid := row.NodeID
		wallet, err := db.OverlayCache().GetWalletAddress(ctx, nid)
		if err != nil {
			return "", err
		}
		row.Wallet = wallet
		record := structToStringSlice(row)
		if err := w.Write(record); err != nil {
			return "", err
		}
	}
	if err := w.Error(); err != nil {
		return "", err
	}
	w.Flush()
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
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
		strconv.FormatInt(s.PutTotal, 10),
		strconv.FormatInt(s.GetTotal, 10),
		s.Date.Format("2006-01-02"),
		s.Wallet,
	}
	return record
}
