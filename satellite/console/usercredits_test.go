// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/currency"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/rewards"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUserCredits(t *testing.T) {
	t.Skip("Skip until usercredits.Create method is cockroach compatible. https://github.com/cockroachdb/cockroach/issues/42881")
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		consoleDB := db.Console()

		user, referrer, activeOffer, defaultOffer := setupData(ctx, t, db)
		randomID := testrand.UUID()
		invalidOffer := rewards.Offer{
			ID: 10,
		}

		// test foreign key constraint for inserting a new user credit entry with randomID
		var invalidUserCredits []console.CreateCredit
		invalid1, err := console.NewCredit(activeOffer, console.Invitee, randomID, &referrer.ID)
		require.NoError(t, err)
		invalid2, err := console.NewCredit(&invalidOffer, console.Invitee, user.ID, &referrer.ID)
		require.NoError(t, err)
		invalid3, err := console.NewCredit(activeOffer, console.Invitee, randomID, &randomID)
		require.NoError(t, err)

		invalidUserCredits = append(invalidUserCredits, *invalid1, *invalid2, *invalid3)

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
			userCredit     console.CreateCredit
			chargedCredits int
			expected       result
		}{
			{
				userCredit: console.CreateCredit{
					OfferInfo: rewards.RedeemOffer{
						RedeemableCap: activeOffer.RedeemableCap,
						Status:        activeOffer.Status,
						Type:          activeOffer.Type,
					},
					UserID:        user.ID,
					OfferID:       activeOffer.ID,
					ReferredBy:    &referrer.ID,
					Type:          console.Invitee,
					CreditsEarned: currency.Cents(100),
					ExpiresAt:     time.Now().UTC().AddDate(0, 1, 0),
				},
				chargedCredits: 120,
				expected: result{
					remainingCharge: 20,
					usage: console.UserCreditUsage{
						AvailableCredits: currency.Cents(0),
						UsedCredits:      currency.Cents(100),
						Referred:         0,
					},
					referred: 0,
				},
			},
			{
				// simulate a credit that's already expired
				userCredit: console.CreateCredit{
					OfferInfo: rewards.RedeemOffer{
						RedeemableCap: activeOffer.RedeemableCap,
						Status:        activeOffer.Status,
						Type:          activeOffer.Type,
					},
					UserID:        user.ID,
					OfferID:       activeOffer.ID,
					ReferredBy:    &referrer.ID,
					Type:          console.Invitee,
					CreditsEarned: currency.Cents(100),
					ExpiresAt:     time.Now().UTC().AddDate(0, 0, -5),
				},
				chargedCredits: 60,
				expected: result{
					remainingCharge: 60,
					usage: console.UserCreditUsage{
						AvailableCredits: currency.Cents(0),
						UsedCredits:      currency.Cents(100),
						Referred:         0,
					},
					referred:     0,
					hasCreateErr: true,
					hasUpdateErr: true,
				},
			},
			{
				// simulate a credit that's not expired
				userCredit: console.CreateCredit{
					OfferInfo: rewards.RedeemOffer{
						RedeemableCap: activeOffer.RedeemableCap,
						Status:        activeOffer.Status,
						Type:          activeOffer.Type,
					},
					UserID:        user.ID,
					OfferID:       activeOffer.ID,
					ReferredBy:    &referrer.ID,
					Type:          console.Invitee,
					CreditsEarned: currency.Cents(100),
					ExpiresAt:     time.Now().UTC().AddDate(0, 0, 5),
				},
				chargedCredits: 80,
				expected: result{
					remainingCharge: 0,
					usage: console.UserCreditUsage{
						AvailableCredits: currency.Cents(20),
						UsedCredits:      currency.Cents(180),
						Referred:         0,
					},
					referred: 0,
				},
			},
			{
				// simulate redeemable capacity has been reached for active offers
				userCredit: console.CreateCredit{
					OfferInfo: rewards.RedeemOffer{
						RedeemableCap: 1,
						Status:        activeOffer.Status,
						Type:          activeOffer.Type,
					},
					UserID:        user.ID,
					OfferID:       activeOffer.ID,
					ReferredBy:    &randomID,
					Type:          console.Invitee,
					CreditsEarned: currency.Cents(100),
					ExpiresAt:     time.Now().UTC().AddDate(0, 1, 0),
				},
				expected: result{
					usage: console.UserCreditUsage{
						Referred:         0,
						AvailableCredits: currency.Cents(20),
						UsedCredits:      currency.Cents(180),
					},
					referred:     0,
					hasCreateErr: true,
				},
			},
			{
				// simulate redeemable capacity has been reached for default offers
				userCredit: console.CreateCredit{
					OfferInfo: rewards.RedeemOffer{
						RedeemableCap: defaultOffer.RedeemableCap,
						Status:        defaultOffer.Status,
						Type:          defaultOffer.Type,
					},
					UserID:        user.ID,
					OfferID:       defaultOffer.ID,
					ReferredBy:    nil,
					Type:          console.Invitee,
					CreditsEarned: currency.Cents(100),
					ExpiresAt:     time.Now().UTC().AddDate(0, 1, 0),
				},
				expected: result{
					usage: console.UserCreditUsage{
						Referred:         0,
						AvailableCredits: currency.Cents(120),
						UsedCredits:      currency.Cents(180),
					},
					referred:     0,
					hasCreateErr: false,
				},
			},
			{
				// simulate credit on account creation
				userCredit: console.CreateCredit{
					OfferInfo: rewards.RedeemOffer{
						RedeemableCap: defaultOffer.RedeemableCap,
						Status:        defaultOffer.Status,
						Type:          defaultOffer.Type,
					},
					UserID:        user.ID,
					OfferID:       defaultOffer.ID,
					ReferredBy:    &referrer.ID,
					Type:          console.Invitee,
					CreditsEarned: currency.Cents(0),
					ExpiresAt:     time.Now().UTC().AddDate(0, 1, 0),
				},
				expected: result{
					usage: console.UserCreditUsage{
						Referred:         0,
						AvailableCredits: currency.Cents(220),
						UsedCredits:      currency.Cents(180),
					},
					referred:     0,
					hasCreateErr: false,
				},
			},
			{
				// simulate credit redemption for referrer
				userCredit: console.CreateCredit{
					OfferInfo: rewards.RedeemOffer{
						RedeemableCap: activeOffer.RedeemableCap,
						Status:        activeOffer.Status,
						Type:          activeOffer.Type,
					},
					UserID:        referrer.ID,
					OfferID:       activeOffer.ID,
					ReferredBy:    nil,
					Type:          console.Referrer,
					CreditsEarned: activeOffer.AwardCredit,
					ExpiresAt:     time.Now().UTC().AddDate(0, 0, activeOffer.AwardCreditDurationDays),
				},
				expected: result{
					usage: console.UserCreditUsage{
						Referred:         1,
						AvailableCredits: activeOffer.AwardCredit,
						UsedCredits:      currency.Cents(0),
					},
					referred:     1,
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
		ID:           testrand.UUID(),
		FullName:     "John Doe",
		Email:        "john@mail.test",
		PasswordHash: userPassHash,
		Status:       console.Active,
	})
	require.NoError(t, err)

	//create an user as referrer
	referrer, err = consoleDB.Users().Insert(ctx, &console.User{
		ID:           testrand.UUID(),
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
