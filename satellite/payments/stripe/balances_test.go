// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/payments/stripe"
)

func TestBalances(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		userID := planet.Uplinks[0].Projects[0].Owner.ID
		balances := sat.API.Payments.Accounts.Balances()

		tx1Amount := int64(1000)
		tx1Desc := "test description 1"

		b, err := balances.ApplyCredit(ctx, userID, tx1Amount, tx1Desc, "")
		require.NoError(t, err)
		require.Equal(t, decimal.NewFromInt(tx1Amount), b.Credits)

		bal, err := balances.Get(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, decimal.NewFromInt(tx1Amount), bal.Credits)

		tx2Amount := int64(-1000)
		tx2Desc := "test description 2"
		endingBalance := tx1Amount + tx2Amount
		b, err = balances.ApplyCredit(ctx, userID, tx2Amount, tx2Desc, "")
		require.NoError(t, err)
		require.Equal(t, decimal.NewFromInt(endingBalance), b.Credits)

		bal, err = balances.Get(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, decimal.NewFromInt(endingBalance), bal.Credits)

		tx3Amount := int64(-1000)
		tx3Desc := "test description 3"
		endingBalance += tx3Amount
		b, err = balances.ApplyCredit(ctx, userID, tx3Amount, tx3Desc, "")
		require.NoError(t, err)
		require.Equal(t, decimal.NewFromInt(endingBalance), b.Credits)

		bal, err = balances.Get(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, decimal.NewFromInt(endingBalance), bal.Credits)

		list, err := balances.ListTransactions(ctx, userID)
		require.NoError(t, err)
		require.Len(t, list, 3)
		require.Equal(t, tx3Amount, list[0].Amount)
		require.Equal(t, tx3Desc, list[0].Description)
		require.Equal(t, tx2Amount, list[1].Amount)
		require.Equal(t, tx2Desc, list[1].Description)
		require.Equal(t, tx1Amount, list[2].Amount)
		require.Equal(t, tx1Desc, list[2].Description)

		b, err = balances.ApplyCredit(ctx, userID, tx2Amount, stripe.MockCBTXsNewFailure, "")
		require.Error(t, err)
		require.Nil(t, b)

		list, err = balances.ListTransactions(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, 3, len(list))
	})
}
