// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/payments"
)

func TestPaymentIntents_ChargeCard(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		req := payments.ChargeCardRequest{
			CardID: "card_123",
			CreateIntentParams: payments.CreateIntentParams{
				UserID:   userID,
				Amount:   100,
				Metadata: map[string]string{"key": "value"},
			},
		}

		resp, err := satellite.API.Payments.Accounts.PaymentIntents().ChargeCard(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.Success)
	})
}
