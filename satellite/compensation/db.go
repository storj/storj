// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"context"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/currency"
)

type EscrowAmounts struct {
	TotalHeld     currency.MicroUnit
	TotalDisposed currency.MicroUnit
}

type DB interface {
	QueryEscrowAmounts(ctx context.Context, nodeID storj.NodeID) (EscrowAmounts, error)
	QueryPayedInYear(ctx context.Context, nodeID storj.NodeID, year int) (currency.MicroUnit, error)
	RecordPeriod(ctx context.Context, paystubs []Paystub, payments []Payment) error
	RecordPayments(ctx context.Context, payments []Payment) error
}
