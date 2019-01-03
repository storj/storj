// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import "storj.io/storj/pkg/pb"

// Constants for accounting_raw, accounting_rollup, and accounting_timestamps
const (
	// AtRest is the data_type representing at-rest data calculated from pointerdb
	BandwithPut       = int(pb.PayerBandwidthAllocation_PUT)
	BandwithGet       = int(pb.PayerBandwidthAllocation_GET)
	BandwithGetAudit  = int(pb.PayerBandwidthAllocation_GET_AUDIT)
	BandwithGetRepair = int(pb.PayerBandwidthAllocation_GET_REPAIR)
	BandwithPutRepair = int(pb.PayerBandwidthAllocation_PUT_REPAIR)
	AtRest            = int(pb.PayerBandwidthAllocation_PUT_REPAIR + 1)
	// LastAtRestTally represents the accounting timestamp for the at-rest data calculation
	LastAtRestTally = "LastAtRestTally"
	// LastBandwidthTally represents the accounting timestamp for the bandwidth allocation query
	LastBandwidthTally = "LastBandwidthTally"
)
