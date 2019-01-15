// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import "storj.io/storj/pkg/pb"

// Constants for accounting_raw, accounting_rollup, and accounting_timestamps
const (
	// AtRest is the data_type representing at-rest data calculated from pointerdb
	BandwidthPut       = int(pb.PayerBandwidthAllocation_PUT)
	BandwidthGet       = int(pb.PayerBandwidthAllocation_GET)
	BandwidthGetAudit  = int(pb.PayerBandwidthAllocation_GET_AUDIT)
	BandwidthGetRepair = int(pb.PayerBandwidthAllocation_GET_REPAIR)
	BandwidthPutRepair = int(pb.PayerBandwidthAllocation_PUT_REPAIR)
	AtRest             = int(pb.PayerBandwidthAllocation_PUT_REPAIR + 1)
	// LastAtRestTally represents the accounting timestamp for the at-rest data calculation
	LastAtRestTally = "LastAtRestTally"
	// LastBandwidthTally represents the accounting timestamp for the bandwidth allocation query
	LastBandwidthTally = "LastBandwidthTally"
	// LastRollup represents the accounting timestamp for rollup calculations
	LastRollup = "LastRollup"
)
