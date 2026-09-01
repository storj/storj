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

// PaystubConflict describes an already recorded paystub that recording another
// one for the same (period, node) would replace.
//
// Distributed and Payments are what make a replacement dangerous: both are a
// record that money moved, while ReplaceNoReturn_StoragenodePaystub overwrites
// the paystub row and leaves the storagenode_payments rows behind. Replacing a
// row that has distributed=X with one that has distributed=0 makes the next
// Prepare carry X over (toDistribute = owed + TotalPaid - TotalDistributed) and
// pay the node a second time.
type PaystubConflict struct {
	Period      Period
	NodeID      storj.NodeID
	Distributed currency.MicroUnit
	Payments    int
}

// PaidOut reports whether the existing paystub records a payout, which is what
// makes replacing it a potential double payout.
func (conflict PaystubConflict) PaidOut() bool {
	return conflict.Distributed.Value() != 0 || conflict.Payments > 0
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

	// RecordPaystubs records a set of paystubs without any payment. A paystub
	// is replaced if one already exists for the (period, node) it is for.
	RecordPaystubs(ctx context.Context, paystubs []Paystub) error

	// QueryPaystubConflicts returns, for each of the given paystubs, the
	// already recorded paystub that RecordPaystubs would replace. Paystubs
	// with nothing recorded for their (period, node) are not in the result.
	QueryPaystubConflicts(ctx context.Context, paystubs []Paystub) ([]PaystubConflict, error)

	// RecordPayments records one off individual payments.
	RecordPayments(ctx context.Context, payments []Payment) error

	// RecordPaymentsWithDistribution records payments and sets the distributed
	// amount of the paystub of every (period, node) it touches to the total of
	// the payments recorded for that period and node. A payment without a
	// paystub for its period and node is an error, and so is the same payment
	// appearing twice in the input.
	RecordPaymentsWithDistribution(ctx context.Context, payments []Payment) error
}
