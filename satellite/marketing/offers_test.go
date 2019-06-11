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

func TestOffer_Database(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// Happy path
		validOffers := []marketing.NewOffer{
			{
				Name:                      "test",
				Description:               "test offer 1",
				AwardCreditInCents:        100,
				InviteeCreditInCents:      50,
				AwardCreditDurationDays:   60,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().UTC().Add(time.Hour * 1),
				Status:                    marketing.Active,
				Type:                      marketing.Referral,
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
				Status:                    marketing.Default,
				Type:                      marketing.FreeCredit,
			},
		}

		for i := range validOffers {
			new, err := planet.Satellites[0].DB.Marketing().Offers().Create(ctx, &validOffers[i])
			require.NoError(t, err)

			all, err := planet.Satellites[0].DB.Marketing().Offers().ListAll(ctx)
			require.NoError(t, err)
			require.Contains(t, all, *new)

			c, err := planet.Satellites[0].DB.Marketing().Offers().GetCurrentByType(ctx, new.Type)
			require.NoError(t, err)
			require.Equal(t, new, c)

			update := &marketing.UpdateOffer{
				ID:        new.ID,
				Status:    marketing.Done,
				ExpiresAt: time.Now(),
			}

			err = planet.Satellites[0].DB.Marketing().Offers().Redeem(ctx, update.ID)
			require.NoError(t, err)

			err = planet.Satellites[0].DB.Marketing().Offers().Finish(ctx, update.ID)
			require.NoError(t, err)

			current, err := planet.Satellites[0].DB.Marketing().Offers().ListAll(ctx)
			if new.Status == marketing.Default {
				require.Equal(t, new.NumRedeemed, current[i].NumRedeemed)
			} else {
				require.Equal(t, new.NumRedeemed+1, current[i].NumRedeemed)
			}
			require.Equal(t, marketing.Done, current[i].Status)
		}

		// create with expired offer
		expiredOffers := []marketing.NewOffer{
			{
				Name:                      "test",
				Description:               "test offer",
				AwardCreditInCents:        100,
				InviteeCreditInCents:      50,
				AwardCreditDurationDays:   60,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().UTC().Add(time.Hour * -1),
				Status:                    marketing.Active,
				Type:                      marketing.FreeCredit,
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
				Status:                    marketing.Default,
				Type:                      marketing.Referral,
			},
		}

		for i := range expiredOffers {
			output, err := planet.Satellites[0].DB.Marketing().Offers().Create(ctx, &expiredOffers[i])
			require.Error(t, err)
			require.Nil(t, output)
		}
	})
}
