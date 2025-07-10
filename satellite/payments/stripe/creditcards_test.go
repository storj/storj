// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	stripeLib "github.com/stripe/stripe-go/v81"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripe"
)

func TestCreditCards_List(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		cards, err := satellite.API.Payments.Accounts.CreditCards().List(ctx, userID)
		require.NoError(t, err)
		require.Zero(t, cards)
	})
}

func TestCreditCards_Add(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		tests := []struct {
			name      string
			cardToken string
			shouldErr bool
		}{
			{
				"success",
				"test",
				false,
			},
			{
				"Add returns error when StripePaymentMethods.New fails",
				stripe.TestPaymentMethodsNewFailure,
				true,
			},
			{
				"Add returns error when StripePaymentMethods.Attach fails",
				stripe.TestPaymentMethodsAttachFailure,
				true,
			},
		}
		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				u, err := satellite.AddUser(ctx, console.CreateUser{
					FullName: "Test User",
					Email:    fmt.Sprintf("t%d@storj.test", i),
				}, 1)
				require.NoError(t, err)

				card, err := satellite.API.Payments.Accounts.CreditCards().Add(ctx, u.ID, tt.cardToken)
				if tt.shouldErr {
					require.Error(t, err)
					require.Empty(t, card)
				} else {
					require.NoError(t, err)
					require.NotEmpty(t, card)
				}

				cards, err := satellite.API.Payments.Accounts.CreditCards().List(ctx, u.ID)
				require.NoError(t, err)

				if !tt.shouldErr {
					require.Len(t, cards, 1)
				} else {
					require.Len(t, cards, 0)
				}
			})
		}
	})
}

func TestCreditCards_AddByPaymentMethodID(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		u, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@storj.test",
		}, 1)
		require.NoError(t, err)

		_, err = satellite.API.Payments.Accounts.CreditCards().AddByPaymentMethodID(ctx, u.ID, "non-existent", false)
		require.Error(t, err)

		// Add expired card to be automatically removed on successful new card addition.
		invalidExpYear := 2000
		invalidExpMonth := 10
		pm, err := satellite.API.Payments.StripeClient.PaymentMethods().New(&stripeLib.PaymentMethodParams{
			Type: stripeLib.String(string(stripeLib.PaymentMethodTypeCard)),
			Card: &stripeLib.PaymentMethodCardParams{
				Token:    stripeLib.String("test"),
				ExpYear:  stripeLib.Int64(int64(invalidExpYear)),
				ExpMonth: stripeLib.Int64(int64(invalidExpMonth)),
			},
		})
		require.NoError(t, err)

		customerID, err := satellite.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, u.ID)
		require.NoError(t, err)

		_, err = satellite.API.Payments.StripeClient.PaymentMethods().Attach(pm.ID, &stripeLib.PaymentMethodAttachParams{
			Params:   stripeLib.Params{Context: ctx},
			Customer: &customerID,
		})
		require.NoError(t, err)

		cards, err := satellite.API.Payments.Accounts.CreditCards().List(ctx, u.ID)
		require.NoError(t, err)
		require.Len(t, cards, 1)
		require.Equal(t, invalidExpYear, cards[0].ExpYear)
		require.Equal(t, invalidExpMonth, cards[0].ExpMonth)

		pm, err = satellite.API.Payments.StripeClient.PaymentMethods().New(&stripeLib.PaymentMethodParams{
			Type: stripeLib.String(string(stripeLib.PaymentMethodTypeCard)),
			Card: &stripeLib.PaymentMethodCardParams{
				Token: stripeLib.String("test1"),
			},
		})
		require.NoError(t, err)

		_, err = satellite.API.Payments.Accounts.CreditCards().AddByPaymentMethodID(ctx, u.ID, pm.ID, false)
		require.NoError(t, err)

		_, err = satellite.API.Payments.Accounts.CreditCards().AddByPaymentMethodID(ctx, u.ID, pm.ID, false)
		require.Error(t, err)
		require.True(t, payments.ErrDuplicateCard.Has(err))

		cards, err = satellite.API.Payments.Accounts.CreditCards().List(ctx, u.ID)
		require.NoError(t, err)
		require.Len(t, cards, 1)
		require.NotEqual(t, invalidExpYear, cards[0].ExpYear)
		require.NotEqual(t, invalidExpMonth, cards[0].ExpMonth)
	})
}

func TestCreditCards_AddDuplicateCard(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		u, err := satellite.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@storj.test",
		}, 1)
		require.NoError(t, err)

		cardToken := "testToken"

		card, err := satellite.API.Payments.Accounts.CreditCards().Add(ctx, u.ID, cardToken)
		require.NoError(t, err)
		require.NotEmpty(t, card)

		card, err = satellite.API.Payments.Accounts.CreditCards().Add(ctx, u.ID, cardToken)
		require.Error(t, err)
		require.True(t, payments.ErrDuplicateCard.Has(err))
		require.Empty(t, card)

		cards, err := satellite.API.Payments.Accounts.CreditCards().List(ctx, u.ID)
		require.NoError(t, err)
		require.Len(t, cards, 1)
	})
}

func TestCreditCards_Remove(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		userID := planet.Uplinks[0].Projects[0].Owner.ID
		user2ID := planet.Uplinks[1].Projects[0].Owner.ID

		card1, err := satellite.API.Payments.Accounts.CreditCards().Add(ctx, userID, "test")
		require.NoError(t, err)

		// card2 becomes the default card
		card2, err := satellite.API.Payments.Accounts.CreditCards().Add(ctx, userID, "test2")
		require.NoError(t, err)

		cards, err := satellite.API.Payments.Accounts.CreditCards().List(ctx, userID)
		require.NoError(t, err)
		require.Len(t, cards, 2)

		// user2ID should not be able to delete userID's cards
		for _, card := range cards {
			err = satellite.API.Payments.Accounts.CreditCards().Remove(ctx, user2ID, card.ID, false)
			require.Error(t, err)
			require.True(t, payments.ErrCardNotFound.Has(err))
		}

		// Can not remove default card
		err = satellite.API.Payments.Accounts.CreditCards().Remove(ctx, userID, card2.ID, false)
		require.Error(t, err)
		require.True(t, payments.ErrDefaultCard.Has(err))

		err = satellite.API.Payments.Accounts.CreditCards().Remove(ctx, userID, card1.ID, false)
		require.NoError(t, err)

		cards, err = satellite.API.Payments.Accounts.CreditCards().List(ctx, userID)
		require.NoError(t, err)
		require.Len(t, cards, 1)
		require.Equal(t, card2, cards[0])
	})
}

func TestCreditCards_Update(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		cardUpdateParams := payments.CardUpdateParams{
			ExpMonth: 10,
			ExpYear:  2025,
		}

		card1, err := satellite.API.Payments.Accounts.CreditCards().Add(ctx, userID, "test")
		require.NoError(t, err)
		require.NotEqualValues(t, cardUpdateParams.ExpMonth, card1.ExpMonth)
		require.NotEqualValues(t, cardUpdateParams.ExpYear, card1.ExpYear)

		err = satellite.API.Payments.Accounts.CreditCards().Update(ctx, userID, cardUpdateParams)
		require.True(t, payments.ErrCardNotFound.Has(err))

		cardUpdateParams.CardID = card1.ID
		err = satellite.API.Payments.Accounts.CreditCards().Update(ctx, userID, cardUpdateParams)
		require.NoError(t, err)

		cards, err := satellite.API.Payments.Accounts.CreditCards().List(ctx, userID)
		require.NoError(t, err)
		require.Len(t, cards, 1)
		require.Equal(t, card1.ID, cards[0].ID)

		card1 = cards[0]
		require.NotEqual(t, cardUpdateParams.ExpMonth, card1.ExpMonth)
		require.EqualValues(t, cardUpdateParams.ExpMonth, card1.ExpMonth)
		require.EqualValues(t, cardUpdateParams.ExpYear, card1.ExpYear)
	})
}

func TestCreditCards_RemoveAll(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		_, err := satellite.API.Payments.Accounts.CreditCards().Add(ctx, userID, "test")
		require.NoError(t, err)

		_, err = satellite.API.Payments.Accounts.CreditCards().Add(ctx, userID, "test2")
		require.NoError(t, err)

		cards, err := satellite.API.Payments.Accounts.CreditCards().List(ctx, userID)
		require.NoError(t, err)
		require.Len(t, cards, 2)

		err = satellite.API.Payments.Accounts.CreditCards().RemoveAll(ctx, userID)
		require.NoError(t, err)

		cards, err = satellite.API.Payments.Accounts.CreditCards().List(ctx, userID)
		require.NoError(t, err)
		require.Len(t, cards, 0)
	})
}
