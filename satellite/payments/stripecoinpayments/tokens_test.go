// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

func TestTokens_ListDepositBonuses(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		// No deposit bonuses expected at this point
		bonuses, err := satellite.API.Payments.Accounts.StorjTokens().ListDepositBonuses(ctx, userID)
		require.NoError(t, err)
		require.Len(t, bonuses, 0)

		customerID, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, userID)
		require.NoError(t, err)

		txID := base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))

		// Credit the Stripe balance with a STORJ deposit of $10
		params := stripe.CustomerBalanceTransactionParams{
			Customer:    stripe.String(customerID),
			Amount:      stripe.Int64(-1000),
			Description: stripe.String(stripecoinpayments.StripeDepositTransactionDescription),
		}
		params.AddMetadata("txID", txID)
		_, err = satellite.API.Payments.Stripe.CustomerBalanceTransactions().New(&params)
		require.NoError(t, err)

		// Credit the Stripe balance with a 13% bonus - $1.30
		params = stripe.CustomerBalanceTransactionParams{
			Customer:    stripe.String(customerID),
			Amount:      stripe.Int64(-130),
			Description: stripe.String(stripecoinpayments.StripeDepositBonusTransactionDescription),
		}
		params.AddMetadata("txID", txID)
		params.AddMetadata("percentage", "13")
		bonusTx, err := satellite.API.Payments.Stripe.CustomerBalanceTransactions().New(&params)
		require.NoError(t, err)

		// Expect the list to contain the 13% bonus
		bonuses, err = satellite.API.Payments.Accounts.StorjTokens().ListDepositBonuses(ctx, userID)
		require.NoError(t, err)
		require.Len(t, bonuses, 1)
		require.EqualValues(t, txID, bonuses[0].TransactionID)
		require.EqualValues(t, 130, bonuses[0].AmountCents)
		require.EqualValues(t, 13, bonuses[0].Percentage)
		require.Equal(t, bonusTx.Created, bonuses[0].CreatedAt.Unix())
	})
}
