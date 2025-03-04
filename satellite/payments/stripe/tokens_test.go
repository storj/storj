// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
	stripe1 "storj.io/storj/satellite/payments/stripe"
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
		txParams := stripe.CustomerBalanceTransactionParams{
			Params:      stripe.Params{Context: ctx},
			Customer:    stripe.String(customerID),
			Amount:      stripe.Int64(-1000),
			Description: stripe.String(stripe1.StripeDepositTransactionDescription),
		}
		txParams.AddMetadata("txID", txID)
		_, err = satellite.API.Payments.StripeClient.CustomerBalanceTransactions().New(&txParams)
		require.NoError(t, err)

		// Credit the Stripe balance with a 13% bonus - $1.30
		txParams = stripe.CustomerBalanceTransactionParams{
			Params:      stripe.Params{Context: ctx},
			Customer:    stripe.String(customerID),
			Amount:      stripe.Int64(-130),
			Description: stripe.String(stripe1.StripeDepositBonusTransactionDescription),
		}
		txParams.AddMetadata("txID", txID)
		txParams.AddMetadata("percentage", "13")
		bonusTx, err := satellite.API.Payments.StripeClient.CustomerBalanceTransactions().New(&txParams)
		require.NoError(t, err)

		// Add migrated deposit bonus to Stripe metadata
		credit := payments.Credit{
			UserID:        userID,
			Amount:        100,
			TransactionID: coinpayments.TransactionID(base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))),
			Created:       time.Now(),
		}
		b, err := json.Marshal(credit)
		require.NoError(t, err)
		customerParams := stripe.CustomerParams{
			Params: stripe.Params{Context: ctx},
			Metadata: map[string]string{
				"credit_" + credit.TransactionID.String(): string(b),
			},
		}
		_, err = satellite.API.Payments.StripeClient.Customers().Update(customerID, &customerParams)
		require.NoError(t, err)

		// Expect the list to contain the 10% bonus from Stripe metadata and 13% bonus from Stripe balance
		bonuses, err = satellite.API.Payments.Accounts.StorjTokens().ListDepositBonuses(ctx, userID)
		require.NoError(t, err)
		require.Len(t, bonuses, 2)
		require.EqualValues(t, credit.TransactionID, bonuses[0].TransactionID)
		require.EqualValues(t, 100, bonuses[0].AmountCents)
		require.EqualValues(t, 10, bonuses[0].Percentage)
		require.Equal(t, bonusTx.Created, bonuses[0].CreatedAt.Unix())
		require.EqualValues(t, txID, bonuses[1].TransactionID)
		require.EqualValues(t, 130, bonuses[1].AmountCents)
		require.EqualValues(t, 13, bonuses[1].Percentage)
		require.Equal(t, bonusTx.Created, bonuses[1].CreatedAt.Unix())
	})
}
