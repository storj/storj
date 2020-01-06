// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"time"

	"storj.io/common/storj"
)

// Constants for accounting_raw, accounting_rollup, and accounting_timestamps
const (
	// LastAtRestTally represents the accounting timestamp for the at-rest data calculation
	LastAtRestTally = "LastAtRestTally"
	// LastBandwidthTally represents the accounting timestamp for the bandwidth allocation query
	LastBandwidthTally = "LastBandwidthTally"
	// LastRollup represents the accounting timestamp for rollup calculations
	LastRollup = "LastRollup"
)

// CSVRow represents data from QueryPaymentInfo without exposing dbx
type CSVRow struct {
	NodeID           storj.NodeID
	NodeCreationDate time.Time
	AtRestTotal      float64
	GetRepairTotal   int64
	PutRepairTotal   int64
	GetAuditTotal    int64
	PutTotal         int64
	GetTotal         int64
	Wallet           string
	Disqualified     *time.Time
}
