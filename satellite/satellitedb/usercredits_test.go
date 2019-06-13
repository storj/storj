// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/marketing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/console"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUsercredits(t *testing.T) {
	t.Parallel()

	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		test(ctx, t, db)
	})
}

func test(ctx context.Context, t *testing.T, store satellite.DB) {
	consoleDB := store.Console()

	user, referrer, offer, err := setupData(ctx, store)
	randomID, err := uuid.New()
	require.NoError(t, err)

	var inValidUserCredits = []struct {
		userCredit console.UserCredit
	}{
		{
			userCredit: console.UserCredit{
				UserID:               *randomID,
				OfferID:              offer.ID,
				ReferredBy:           referrer.ID,
				CreditsEarnedInCents: 100,
				ExpiresAt:            time.Now().UTC().AddDate(0, 1, 0),
			},
		},
		{
			userCredit: console.UserCredit{
				UserID:               user.ID,
				OfferID:              10,
				ReferredBy:           referrer.ID,
				CreditsEarnedInCents: 100,
				ExpiresAt:            time.Now().UTC().AddDate(0, 1, 0),
			},
		},
		{
			userCredit: console.UserCredit{
				UserID:               user.ID,
				OfferID:              offer.ID,
				ReferredBy:           *randomID,
				CreditsEarnedInCents: 100,
				ExpiresAt:            time.Now().UTC().AddDate(0, 1, 0),
			},
		},
	}

	for _, ivc := range inValidUserCredits {
		_, err := consoleDB.UserCredits().Create(ctx, ivc.userCredit)
		require.Error(t, err)
	}

	var validUserCredits = []struct {
		userCredit     console.UserCredit
		chargedCredits int
		expected       int
	}{
		{
			userCredit: console.UserCredit{
				UserID:               user.ID,
				OfferID:              offer.ID,
				ReferredBy:           referrer.ID,
				CreditsEarnedInCents: 100,
				ExpiresAt:            time.Now().UTC().AddDate(0, 1, 0),
			},
			chargedCredits: 100,
			expected:       0,
		},
		{
			userCredit: console.UserCredit{
				UserID:               user.ID,
				OfferID:              offer.ID,
				ReferredBy:           referrer.ID,
				CreditsEarnedInCents: 100,
				ExpiresAt:            time.Now().UTC().AddDate(0, 0, -5),
			},
			chargedCredits: 60,
			expected:       0,
		},
	}

	for i, vc := range validUserCredits {
		_, err = consoleDB.UserCredits().Create(ctx, vc.userCredit)
		require.NoError(t, err)

		{
			referredCount, err := consoleDB.UserCredits().TotalReferredCount(ctx, vc.userCredit.ReferredBy)
			if err != nil {
				require.True(t, uuid.Equal(*randomID, vc.userCredit.ReferredBy))
				continue
			}
			require.NoError(t, err)
			require.Equal(t, int64(i+1), referredCount)
		}

		{
			err := consoleDB.UserCredits().UpdateAvailableCredits(ctx, vc.chargedCredits, vc.userCredit.UserID, time.Now().UTC())
			require.NoError(t, err)
		}

		{
			availableCredits, err := consoleDB.UserCredits().AvailableCredits(ctx, vc.userCredit.UserID, time.Now().UTC())
			require.NoError(t, err)
			var sum int
			for i := range availableCredits {
				sum += availableCredits[i].CreditsEarnedInCents - availableCredits[i].CreditsUsedInCents
			}

			require.NoError(t, err)
			require.Equal(t, vc.expected, sum)
		}
	}
}

func setupData(ctx context.Context, store satellite.DB) (user *console.User, referrer *console.User, offer *marketing.Offer, err error) {
	consoleDB := store.Console()
	marketingDB := store.Marketing()
	// create user
	var userPassHash [8]byte
	_, err = rand.Read(userPassHash[:])

	var referrerPassHash [8]byte
	_, err = rand.Read(referrerPassHash[:])

	user, err = consoleDB.Users().Insert(ctx, &console.User{
		FullName:     "John Doe",
		Email:        "john@example.com",
		PasswordHash: userPassHash[:],
		Status:       console.Active,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	referrer, err = consoleDB.Users().Insert(ctx, &console.User{
		FullName:     "referrer",
		Email:        "referrer@example.com",
		PasswordHash: referrerPassHash[:],
		Status:       console.Active,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	// create offer
	offer, err = marketingDB.Offers().Create(ctx, &marketing.NewOffer{
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
	})
	if err != nil {
		return nil, nil, nil, err
	}

	return user, referrer, offer, nil
}
