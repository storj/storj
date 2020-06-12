// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

func TestService_MigrateCredits(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		for i, tt := range []struct {
			credits  []int64
			spedings []int64
		}{
			{ // user with no credits and spendings
			},
			{ // user with a single credit, but no spendings
				credits: []int64{1000},
			},
			{ // user with a single credit and spending, some remaining balances
				credits:  []int64{1000},
				spedings: []int64{700},
			},
			{ // user with a single credit and spending, no remaining balances
				credits:  []int64{1000},
				spedings: []int64{1000},
			},
			{ // user with a single credit and spending, negative balances
				credits:  []int64{1000},
				spedings: []int64{2000},
			},
			{ // user with a few credits and spendings, some remaining balance
				credits:  []int64{100, 200, 300},
				spedings: []int64{50, 150},
			},
			{ // user with a few credits and spendings, no remaining balance
				credits:  []int64{100, 200, 300},
				spedings: []int64{50, 150, 400},
			},
			{ // user with a few credits and spendings, negative remaining balance
				credits:  []int64{100, 200},
				spedings: []int64{50, 150, 200},
			},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			user, err := satellite.AddUser(ctx, "testuser"+strconv.Itoa(i), "test@test"+strconv.Itoa(i), 1)
			require.NoError(t, err, errTag)

			project, err := satellite.AddProject(ctx, user.ID, "testproject-"+strconv.Itoa(i))
			require.NoError(t, err, errTag)

			// Keep track of the balance when adding all credits and spendings to the database
			var initialBalance int64

			// Add the credits to the database
			for i, credit := range tt.credits {
				initialBalance += credit
				err = satellite.DB.StripeCoinPayments().Credits().InsertCredit(ctx, payments.Credit{
					UserID:        user.ID,
					TransactionID: coinpayments.TransactionID(user.ID.String() + "-" + strconv.Itoa(i)),
					Amount:        credit,
					Created:       time.Now().UTC(),
				})
				require.NoError(t, err, errTag)
			}

			// Add the credit spendings to the database
			for _, spending := range tt.spedings {
				initialBalance -= spending
				err = satellite.DB.StripeCoinPayments().Credits().InsertCreditsSpending(ctx, stripecoinpayments.CreditsSpending{
					ID:        testrand.UUID(),
					ProjectID: project.ID,
					UserID:    user.ID,
					Amount:    spending,
					Status:    stripecoinpayments.CreditsSpendingStatusApplied,
					Period:    time.Now().UTC(),
					Created:   time.Now().UTC(),
				})
				require.NoError(t, err, errTag)
			}

			// Check that the initial credits balance is as expected
			balance, err := satellite.DB.StripeCoinPayments().Credits().Balance(ctx, user.ID)
			require.NoError(t, err, errTag)
			require.EqualValues(t, initialBalance, balance, errTag)

			// Migrate the credits to Stripe
			err = satellite.API.Payments.Service.MigrateCredits(ctx)
			require.NoError(t, err, errTag)

			// Check that the credits balance in database is now zero.
			balance, err = satellite.DB.StripeCoinPayments().Credits().Balance(ctx, user.ID)
			require.NoError(t, err, errTag)
			if initialBalance > 0 {
				require.Zero(t, balance, errTag)
			} else {
				// If the initial balance was negative (should not be possible in reality) then after migration it remains the same
				require.Equal(t, initialBalance, balance, errTag)
			}

			customerID, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, user.ID)
			require.NoError(t, err, errTag)

			it := satellite.API.Payments.Stripe.CustomerBalanceTransactions().List(&stripe.CustomerBalanceTransactionListParams{Customer: stripe.String(customerID)})
			if initialBalance > 0 {
				// Check that we have only one balance transaction in Stripe, which is the one adding the unspent credits balance
				require.True(t, it.Next(), errTag)
				cbt := it.CustomerBalanceTransaction()
				require.Equal(t, stripe.CustomerBalanceTransactionTypeAdjustment, cbt.Type, errTag)
				require.Equal(t, stripecoinpayments.StripeMigratedDepositBonusTransactionDescription, cbt.Description, errTag)
				require.EqualValues(t, -initialBalance, cbt.Amount, errTag)
			}
			require.False(t, it.Next(), errTag)

			customer, err := satellite.API.Payments.Stripe.Customers().Get(customerID, nil)
			require.NoError(t, err, errTag)
			if len(tt.credits) == 0 {
				require.Nil(t, customer.Metadata, errTag)
			} else {
				// Check that the Stripe customer metadata contains the history of added credits
				require.NotNil(t, customer.Metadata, errTag)
				require.Len(t, customer.Metadata, len(tt.credits), errTag)
				for i, credit := range tt.credits {
					txID := user.ID.String() + "-" + strconv.Itoa(i)
					require.Contains(t, customer.Metadata, "credit_"+txID, errTag)
					require.Contains(t, customer.Metadata["credit_"+txID], `"credit":`+strconv.FormatInt(credit, 10), errTag)
				}
			}
		}
	})
}
