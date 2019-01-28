// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"time"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Constants for accounting_raw, accounting_rollup, and accounting_timestamps
const (
	// AtRest is the data_type representing at-rest data calculated from pointerdb
	BandwidthPut       = int(pb.BandwidthAction_PUT)
	BandwidthGet       = int(pb.BandwidthAction_GET)
	BandwidthGetAudit  = int(pb.BandwidthAction_GET_AUDIT)
	BandwidthGetRepair = int(pb.BandwidthAction_GET_REPAIR)
	BandwidthPutRepair = int(pb.BandwidthAction_PUT_REPAIR)
	AtRest             = int(pb.BandwidthAction_PUT_REPAIR + 1)
	// LastAtRestTally represents the accounting timestamp for the at-rest data calculation
	LastAtRestTally = "LastAtRestTally"
	// LastBandwidthTally represents the accounting timestamp for the bandwidth allocation query
	LastBandwidthTally = "LastBandwidthTally"
	// LastRollup represents the accounting timestamp for rollup calculations
	LastRollup = "LastRollup"
)

// CSVRow represents data from QueryPaymentInfo without exposing dbx
type CSVRow struct {
	NodeID            storj.NodeID
	NodeCreationDate  time.Time
	AuditSuccessRatio float64
	AtRestTotal       float64
	GetRepairTotal    int64
	PutRepairTotal    int64
	GetAuditTotal     int64
	PutTotal          int64
	GetTotal          int64
	Date              time.Time
	Wallet            string
}
