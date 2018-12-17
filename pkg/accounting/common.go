// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

// Constants for accounting_raw and accounting_rollup
const (
	// AtRest is the data_type representing at-rest data calculated from pointerdb
	AtRest = iota
	// Bandwidth is the data_type representing bandwidth allocation.
	Bandwith = iota
	// LastAtRestTally
	LastAtRestTally = "LastAtRestTally"
	// LastBandwidthTally
	LastBandwidthTally = "LastBandwidthTally"
)
