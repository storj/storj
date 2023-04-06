// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
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

func TestCreditCards_Remove(t *testing.T) {
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

		// Save card that should remain after deletion.
		savedCard := cards[0]

		err = satellite.API.Payments.Accounts.CreditCards().Remove(ctx, userID, cards[1].ID)
		require.NoError(t, err)

		cards, err = satellite.API.Payments.Accounts.CreditCards().List(ctx, userID)
		require.NoError(t, err)
		require.Len(t, cards, 1)
		require.Equal(t, savedCard, cards[0])
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
