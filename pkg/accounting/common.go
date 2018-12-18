// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

// Constants for accounting_raw, accounting_rollup, and accounting_timestamps
const (
	// AtRest is the data_type representing at-rest data calculated from pointerdb
	AtRest = iota
	// Bandwidth is the data_type representing bandwidth allocation.
	Bandwith = iota
	// LastAtRestTally represents the accounting timestamp for the at-rest data calculation
	LastAtRestTally = "LastAtRestTally"
	// LastBandwidthTally represents the accounting timestamp for the bandwidth allocation query
	LastBandwidthTally = "LastBandwidthTally"
)
