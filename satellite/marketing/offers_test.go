// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package marketing_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/satellite/marketing"
)

func TestCreateAndListAllOffers(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		newOffer := &marketing.NewOffer{
			Name:                      "test",
			Description:               "test offer",
			Type:                      marketing.Referral,
			CreditInCents:             100,
			AwardCreditDurationDays:   60,
			InviteeCreditDurationDays: 30,
			RedeemableCap:             50,
		}
		createdOffer, err := planet.Satellites[0].DB.Marketing().Offers().Create(ctx, newOffer)
		require.NoError(t, err)
		require.Equal(t, newOffer, createdOffer)
		output, err := planet.Satellites[0].DB.Marketing().Offers().ListAllOffers(ctx)
		require.Contains(t, output, newOffer)
		require.NoError(t, err)
	})
}
