// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"context"

	"storj.io/common/storj"
	"storj.io/storj/private/currency"
)

// TotalAmounts holds the amounts held and disposed.
//
// Invariants:
//
//	TotalHeld >= TotalDisposed
//	TotalPaid >= TotalDisposed
//	TotalPaid >= TotalDistributed (we may distribute less due to minimum payout threshold)
type TotalAmounts struct {
	TotalHeld        currency.MicroUnit // portion from owed that was held back
	TotalDisposed    currency.MicroUnit // portion from held back that went into paid
	TotalPaid        currency.MicroUnit // earned amount that is available to be distributed
	TotalDistributed currency.MicroUnit // amount actually transferred to the operator
}

// DB is the interface we need to source the data to calculate compensation.
type DB interface {
	// QueryTotalAmounts queries the WithheldAmounts for the given nodeID.
	// If genesis is non-nil, only paystubs of that period or later are
	// aggregated; a node with no matching paystubs yields zero totals.
	QueryTotalAmounts(ctx context.Context, nodeID storj.NodeID, genesis *Period) (TotalAmounts, error)

	// QueryAllTotalAmounts queries the WithheldAmounts for every node with at
	// least one paystub row in a single aggregate query. Nodes without any
	// paystubs will not appear in the returned map. If genesis is non-nil,
	// only paystubs of that period or later are aggregated.
	QueryAllTotalAmounts(ctx context.Context, genesis *Period) (map[storj.NodeID]TotalAmounts, error)

	// RecordPeriod records a set of paystubs and payments for some time period.
	RecordPeriod(ctx context.Context, paystubs []Paystub, payments []Payment) error

	// RecordPayments records one off individual payments.
	RecordPayments(ctx context.Context, payments []Payment) error
}
