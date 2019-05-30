// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package marketing_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/satellite/marketing"
)

func TestOfferCycleSuccess(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		offers := []*marketing.NewOffer{
			{
				Name:                      "test",
				Description:               "test offer",
				AwardCreditInCents:        100,
				InviteeCreditInCents:      50,
				AwardCreditDurationDays:   60,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().Add(time.Hour * 1),
				Status:                    marketing.Active,
			},
			{
				Name:                      "test",
				Description:               "test offer",
				AwardCreditInCents:        100,
				InviteeCreditInCents:      50,
				AwardCreditDurationDays:   60,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().Add(time.Hour * 1),
				Status:                    marketing.Default,
			},
		}

		for _, o := range offers {
			new, err := planet.Satellites[0].DB.Marketing().Offers().Create(ctx, o)
			require.NoError(t, err)

			all, err := planet.Satellites[0].DB.Marketing().Offers().ListAll(ctx)
			require.NoError(t, err)
			require.Contains(t, all, *new)

			c, err := planet.Satellites[0].DB.Marketing().Offers().GetCurrent(ctx, new.Status)
			require.NoError(t, err)
			require.Equal(t, new, c)

			update := &marketing.UpdateOffer{
				ID:          new.ID,
				Status:      marketing.Done,
				NumRedeemed: new.NumRedeemed,
				ExpiresAt:   time.Now(),
			}
			err = planet.Satellites[0].DB.Marketing().Offers().Update(ctx, update)
			require.NoError(t, err)
		}
	})
}
