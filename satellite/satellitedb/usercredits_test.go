// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/currency"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/rewards"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUsercredits(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		consoleDB := db.Console()

		user, referrer, activeOffer, defaultOffer := setupData(ctx, t, db)
		randomID := testrand.UUID()

		// test foreign key constraint for inserting a new user credit entry with randomID
		var invalidUserCredits = []console.UserCredit{
			{
				UserID:        randomID,
				OfferID:       activeOffer.ID,
				ReferredBy:    &referrer.ID,
				CreditsEarned: currency.Cents(100),
				ExpiresAt:     time.Now().UTC().AddDate(0, 1, 0),
			},
			{
				UserID:        user.ID,
				OfferID:       10,
				ReferredBy:    &referrer.ID,
				CreditsEarned: currency.Cents(100),
				ExpiresAt:     time.Now().UTC().AddDate(0, 1, 0),
			},
			{
				UserID:        user.ID,
				OfferID:       activeOffer.ID,
				ReferredBy:    &randomID,
				CreditsEarned: currency.Cents(100),
				ExpiresAt:     time.Now().UTC().AddDate(0, 1, 0),
			},
		}

		for _, ivc := range invalidUserCredits {
			err := consoleDB.UserCredits().Create(ctx, ivc)
			require.Error(t, err)
		}

		type result struct {
			remainingCharge int
			usage           console.UserCreditUsage
			referred        int64
			hasUpdateErr    bool
			hasCreateErr    bool
		}

		var validUserCredits = []struct {
			userCredit     console.UserCredit
			chargedCredits int
			redeemableCap  int
			expected       result
		}{
			{
				userCredit: console.UserCredit{
					UserID:        user.ID,
					OfferID:       activeOffer.ID,
					ReferredBy:    &referrer.ID,
					CreditsEarned: currency.Cents(100),
					ExpiresAt:     time.Now().UTC().AddDate(0, 1, 0),
				},
				chargedCredits: 120,
				redeemableCap:  activeOffer.RedeemableCap,
				expected: result{
					remainingCharge: 20,
					usage: console.UserCreditUsage{
						AvailableCredits: currency.Cents(0),
						UsedCredits:      currency.Cents(100),
						Referred:         0,
					},
					referred: 1,
				},
			},
			{
				// simulate a credit that's already expired
				userCredit: console.UserCredit{
					UserID:        user.ID,
					OfferID:       activeOffer.ID,
					ReferredBy:    &referrer.ID,
					CreditsEarned: currency.Cents(100),
					ExpiresAt:     time.Now().UTC().AddDate(0, 0, -5),
				},
				chargedCredits: 60,
				redeemableCap:  activeOffer.RedeemableCap,
				expected: result{
					remainingCharge: 60,
					usage: console.UserCreditUsage{
						AvailableCredits: currency.Cents(0),
						UsedCredits:      currency.Cents(100),
						Referred:         0,
					},
					referred:     1,
					hasCreateErr: true,
					hasUpdateErr: true,
				},
			},
			{
				// simulate a credit that's not expired
				userCredit: console.UserCredit{
					UserID:        user.ID,
					OfferID:       activeOffer.ID,
					ReferredBy:    &referrer.ID,
					CreditsEarned: currency.Cents(100),
					ExpiresAt:     time.Now().UTC().AddDate(0, 0, 5),
				},
				chargedCredits: 80,
				redeemableCap:  activeOffer.RedeemableCap,
				expected: result{
					remainingCharge: 0,
					usage: console.UserCreditUsage{
						AvailableCredits: currency.Cents(20),
						UsedCredits:      currency.Cents(180),
						Referred:         0,
					},
					referred: 2,
				},
			},
			{
				// simulate redeemable capacity has been reached for active offers
				userCredit: console.UserCredit{
					UserID:        user.ID,
					OfferID:       activeOffer.ID,
					ReferredBy:    &randomID,
					CreditsEarned: currency.Cents(100),
					ExpiresAt:     time.Now().UTC().AddDate(0, 1, 0),
				},
				redeemableCap: 1,
				expected: result{
					usage: console.UserCreditUsage{
						Referred:         0,
						AvailableCredits: currency.Cents(20),
						UsedCredits:      currency.Cents(180),
					},
					referred:     2,
					hasCreateErr: true,
				},
			},
			{
				// simulate redeemable capacity has been reached for default offers
				userCredit: console.UserCredit{
					UserID:        user.ID,
					OfferID:       defaultOffer.ID,
					ReferredBy:    nil,
					CreditsEarned: currency.Cents(100),
					ExpiresAt:     time.Now().UTC().AddDate(0, 1, 0),
				},
				redeemableCap: 1,
				expected: result{
					usage: console.UserCreditUsage{
						Referred:         0,
						AvailableCredits: currency.Cents(120),
						UsedCredits:      currency.Cents(180),
					},
					referred:     2,
					hasCreateErr: false,
				},
			},
			{
				// simulate credit on account creation
				userCredit: console.UserCredit{
					UserID:        user.ID,
					OfferID:       defaultOffer.ID,
					ReferredBy:    &referrer.ID,
					CreditsEarned: currency.Cents(0),
					ExpiresAt:     time.Now().UTC().AddDate(0, 1, 0),
				},
				redeemableCap: 0,
				expected: result{
					usage: console.UserCreditUsage{
						Referred:         0,
						AvailableCredits: currency.Cents(220),
						UsedCredits:      currency.Cents(180),
					},
					referred:     3,
					hasCreateErr: false,
				},
			},
		}

		for _, vc := range validUserCredits {
			err := consoleDB.UserCredits().Create(ctx, vc.userCredit)
			if vc.expected.hasCreateErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if vc.userCredit.CreditsEarned.Cents() == 0 {
				err = consoleDB.UserCredits().UpdateEarnedCredits(ctx, vc.userCredit.UserID)
				require.NoError(t, err)
			}

			{
				remainingCharge, err := consoleDB.UserCredits().UpdateAvailableCredits(ctx, vc.chargedCredits, vc.userCredit.UserID, time.Now().UTC())
				if vc.expected.hasUpdateErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				require.Equal(t, vc.expected.remainingCharge, remainingCharge)
			}

			{
				usage, err := consoleDB.UserCredits().GetCreditUsage(ctx, vc.userCredit.UserID, time.Now().UTC())
				require.NoError(t, err)
				require.Equal(t, vc.expected.usage, *usage)
			}

			{
				referred, err := consoleDB.UserCredits().GetCreditUsage(ctx, referrer.ID, time.Now().UTC())
				require.NoError(t, err)
				require.Equal(t, vc.expected.referred, referred.Referred)
			}
		}
	})
}

func setupData(ctx context.Context, t *testing.T, db satellite.DB) (user *console.User, referrer *console.User, activeOffer *rewards.Offer, defaultOffer *rewards.Offer) {
	consoleDB := db.Console()
	offersDB := db.Rewards()

	// create user
	userPassHash := testrand.Bytes(8)
	referrerPassHash := testrand.Bytes(8)

	var err error

	// create an user
	user, err = consoleDB.Users().Insert(ctx, &console.User{
		FullName:     "John Doe",
		Email:        "john@mail.test",
		PasswordHash: userPassHash,
		Status:       console.Active,
	})
	require.NoError(t, err)

	//create an user as referrer
	referrer, err = consoleDB.Users().Insert(ctx, &console.User{
		FullName:     "referrer",
		Email:        "referrer@mail.test",
		PasswordHash: referrerPassHash,
		Status:       console.Active,
	})
	require.NoError(t, err)

	// create an active offer
	activeOffer, err = offersDB.Create(ctx, &rewards.NewOffer{
		Name:                      "active",
		Description:               "active offer",
		AwardCredit:               currency.Cents(100),
		InviteeCredit:             currency.Cents(50),
		AwardCreditDurationDays:   60,
		InviteeCreditDurationDays: 30,
		RedeemableCap:             50,
		ExpiresAt:                 time.Now().UTC().Add(time.Hour * 1),
		Status:                    rewards.Active,
		Type:                      rewards.Referral,
	})
	require.NoError(t, err)

	// create a default offer
	defaultOffer, err = offersDB.Create(ctx, &rewards.NewOffer{
		Name:                      "default",
		Description:               "default offer",
		AwardCredit:               currency.Cents(0),
		InviteeCredit:             currency.Cents(100),
		AwardCreditDurationDays:   0,
		InviteeCreditDurationDays: 14,
		RedeemableCap:             0,
		ExpiresAt:                 time.Now().UTC().Add(time.Hour * 1),
		Status:                    rewards.Default,
		Type:                      rewards.FreeCredit,
	})
	require.NoError(t, err)

	return user, referrer, activeOffer, defaultOffer
}
