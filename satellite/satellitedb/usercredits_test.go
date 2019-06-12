package satellitedb_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

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
	require.NoError(t, err)

	userCredit := console.UserCredit{
		UserID:               user.ID,
		OfferID:              offer.ID,
		ReferredBy:           referrer.ID,
		CreditsEarnedInCents: 100,
		ExpiresAt:            time.Now().UTC().AddDate(0, 0, 1),
	}

	_, err = consoleDB.UserCredits().Create(ctx, userCredit)
	require.NoError(t, err)

	{
		referredCount, err := consoleDB.UserCredits().TotalReferredCount(ctx, referrer.ID)
		require.NoError(t, err)
		require.Equal(t, int64(1), referredCount)
	}

	{
		availableCredits, err := consoleDB.UserCredits().AvailableCredits(ctx, user.ID, time.Now().UTC())
		require.NoError(t, err)
		var sum int
		for i := range availableCredits {
			sum += (availableCredits[i].CreditsEarnedInCents - availableCredits[i].CreditsUsedInCents)
		}

		require.Equal(t, userCredit.CreditsEarnedInCents, sum)
	}

	{

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
		return
	}

	referrer, err = consoleDB.Users().Insert(ctx, &console.User{
		FullName:     "referrer",
		Email:        "referrer@example.com",
		PasswordHash: referrerPassHash[:],
		Status:       console.Active,
	})
	if err != nil {
		return
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
		return
	}

	return
}
