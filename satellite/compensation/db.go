// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"context"

	"storj.io/common/storj"
	"storj.io/storj/private/currency"
)

// WithheldAmounts holds the amounts held and disposed.
type WithheldAmounts struct {
	TotalHeld     currency.MicroUnit
	TotalDisposed currency.MicroUnit
}

// DB is the interface we need to source the data to calculate compensation.
type DB interface {
	// QueryWithheldAmounts queries the WithheldAmounts for the given nodeID.
	QueryWithheldAmounts(ctx context.Context, nodeID storj.NodeID) (WithheldAmounts, error)

	// QueryPaidInYear returns the total amount paid to the nodeID in the provided year.
	QueryPaidInYear(ctx context.Context, nodeID storj.NodeID, year int) (currency.MicroUnit, error)

	// RecordPeriod records a set of paystubs and payments for some time period.
	RecordPeriod(ctx context.Context, paystubs []Paystub, payments []Payment) error

	// RecordPayments records one off individual payments.
	RecordPayments(ctx context.Context, payments []Payment) error
}
