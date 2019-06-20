// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package offers_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/satellite/offers"
)

func TestOffer_Database(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// Happy path
		validOffers := []offers.NewOffer{
			{
				Name:                      "test",
				Description:               "test offer 1",
				AwardCreditInCents:        100,
				InviteeCreditInCents:      50,
				AwardCreditDurationDays:   60,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().UTC().Add(time.Hour * 1),
				Status:                    offers.Active,
				Type:                      offers.Referral,
			},
			{
				Name:                      "test",
				Description:               "test offer 2",
				AwardCreditInCents:        100,
				InviteeCreditInCents:      50,
				AwardCreditDurationDays:   60,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().UTC().Add(time.Hour * 1),
				Status:                    offers.Default,
				Type:                      offers.FreeCredit,
			},
		}

		for i := range validOffers {
			new, err := planet.Satellites[0].DB.Offers().Create(ctx, &validOffers[i])
			require.NoError(t, err)

			all, err := planet.Satellites[0].DB.Offers().ListAll(ctx)
			require.NoError(t, err)
			require.Contains(t, all, *new)

			c, err := planet.Satellites[0].DB.Offers().GetCurrentByType(ctx, new.Type)
			require.NoError(t, err)
			require.Equal(t, new, c)

			update := &offers.UpdateOffer{
				ID:        new.ID,
				Status:    offers.Done,
				ExpiresAt: time.Now(),
			}

			err = planet.Satellites[0].DB.Offers().Redeem(ctx, update.ID)
			require.NoError(t, err)

			err = planet.Satellites[0].DB.Offers().Finish(ctx, update.ID)
			require.NoError(t, err)

			current, err := planet.Satellites[0].DB.Offers().ListAll(ctx)
			require.NoError(t, err)
			if new.Status == offers.Default {
				require.Equal(t, new.NumRedeemed, current[i].NumRedeemed)
			} else {
				require.Equal(t, new.NumRedeemed+1, current[i].NumRedeemed)
			}
			require.Equal(t, offers.Done, current[i].Status)
		}

		// create with expired offer
		expiredOffers := []offers.NewOffer{
			{
				Name:                      "test",
				Description:               "test offer",
				AwardCreditInCents:        100,
				InviteeCreditInCents:      50,
				AwardCreditDurationDays:   60,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().UTC().Add(time.Hour * -1),
				Status:                    offers.Active,
				Type:                      offers.FreeCredit,
			},
			{
				Name:                      "test",
				Description:               "test offer",
				AwardCreditInCents:        100,
				InviteeCreditInCents:      50,
				AwardCreditDurationDays:   60,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().UTC().Add(time.Hour * -1),
				Status:                    offers.Default,
				Type:                      offers.Referral,
			},
		}

		for i := range expiredOffers {
			output, err := planet.Satellites[0].DB.Offers().Create(ctx, &expiredOffers[i])
			require.Error(t, err)
			require.Nil(t, output)
		}
	})
}
