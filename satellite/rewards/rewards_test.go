// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package rewards_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/currency"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/satellite/rewards"
)

func TestOffer_Database(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// Happy path
		validOffers := []rewards.NewOffer{
			{
				Name:                      "test",
				Description:               "test offer 1",
				AwardCredit:               currency.Cents(100),
				InviteeCredit:             currency.Cents(50),
				AwardCreditDurationDays:   60,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().UTC().Add(time.Hour * 1),
				Status:                    rewards.Active,
				Type:                      rewards.Referral,
			},
			{
				Name:                      "test",
				Description:               "test offer 2",
				AwardCredit:               currency.Cents(0),
				InviteeCredit:             currency.Cents(50),
				AwardCreditDurationDays:   0,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().UTC().Add(time.Hour * 1),
				Status:                    rewards.Active,
				Type:                      rewards.FreeCredit,
			},
			{
				Name:                      "Zenko",
				Description:               "partner offer",
				AwardCredit:               currency.Cents(0),
				InviteeCredit:             currency.Cents(50),
				AwardCreditDurationDays:   0,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().UTC().Add(time.Hour * 1),
				Status:                    rewards.Active,
				Type:                      rewards.Partner,
			},
		}

		for i := range validOffers {
			new, err := planet.Satellites[0].DB.Rewards().Create(ctx, &validOffers[i])
			require.NoError(t, err)

			all, err := planet.Satellites[0].DB.Rewards().ListAll(ctx)
			require.NoError(t, err)
			require.Contains(t, all, *new)

			offers, err := planet.Satellites[0].DB.Rewards().GetActiveOffersByType(ctx, new.Type)
			require.NoError(t, err)
			var pID string
			if new.Type == rewards.Partner {
				pID, err = rewards.GetPartnerID(new.Name)
				require.NoError(t, err)
			}
			c, err := offers.GetActiveOffer(new.Type, pID)
			require.NoError(t, err)
			require.Equal(t, new, c)

			err = planet.Satellites[0].DB.Rewards().Finish(ctx, all[i].ID)
			require.NoError(t, err)

			updated, err := planet.Satellites[0].DB.Rewards().ListAll(ctx)
			require.NoError(t, err)
			require.Equal(t, rewards.Done, updated[i].Status)
		}

		// create with expired offer
		expiredOffers := []rewards.NewOffer{
			{
				Name:                      "test",
				Description:               "test offer",
				AwardCredit:               currency.Cents(0),
				InviteeCredit:             currency.Cents(50),
				AwardCreditDurationDays:   0,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             50,
				ExpiresAt:                 time.Now().UTC().Add(time.Hour * -1),
				Status:                    rewards.Active,
				Type:                      rewards.FreeCredit,
			},
			{
				Name:                      "test",
				Description:               "test offer",
				AwardCredit:               currency.Cents(100),
				InviteeCredit:             currency.Cents(50),
				AwardCreditDurationDays:   60,
				InviteeCreditDurationDays: 30,
				RedeemableCap:             0,
				ExpiresAt:                 time.Now().UTC().Add(time.Hour * -1),
				Status:                    rewards.Default,
				Type:                      rewards.Referral,
			},
		}

		for i := range expiredOffers {
			output, err := planet.Satellites[0].DB.Rewards().Create(ctx, &expiredOffers[i])
			require.Error(t, err)
			require.Nil(t, output)
		}
	})
}
