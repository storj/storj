// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUpdateSameAppliesDoesNotExplode(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		tdb := db.StripeCoinPayments().Transactions()
		assert.NoError(t, tdb.Update(ctx, nil, coinpayments.TransactionIDList{"blah", "blah"}))
		assert.NoError(t, tdb.Update(ctx, nil, coinpayments.TransactionIDList{"blah", "blah"}))
	})
}
