// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
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
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		err := satellite.API.Payments.Accounts.CreditCards().Add(ctx, userID, "test")
		require.NoError(t, err)

		cards, err := satellite.API.Payments.Accounts.CreditCards().List(ctx, userID)
		require.NoError(t, err)
		require.Len(t, cards, 1)
	})
}

func TestCreditCards_Remove(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		err := satellite.API.Payments.Accounts.CreditCards().Add(ctx, userID, "test")
		require.NoError(t, err)

		err = satellite.API.Payments.Accounts.CreditCards().Add(ctx, userID, "test2")
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

		err := satellite.API.Payments.Accounts.CreditCards().Add(ctx, userID, "test")
		require.NoError(t, err)

		err = satellite.API.Payments.Accounts.CreditCards().Add(ctx, userID, "test2")
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
