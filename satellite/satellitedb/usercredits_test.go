package satellitedb_test

import (
	"context"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
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

		test(ctx, t, db.Console().UserCredits())
	})
}

func test(ctx context.Context, t *testing.T, store console.UserCredits) {
	// Create
	userID, err := uuid.New()
	referrerID, err := uuid.New()
	require.NoError(t, err)

	userCredit := console.UserCredit{
		UserID:               *userID,
		OfferID:              1,
		ReferredBy:           *referrerID,
		CreditsEarnedInCents: 100,
		ExpiresAt:            time.Now().UTC().AddDate(0, 0, 1),
	}

	createdUser, err := store.Create(ctx, userCredit)
	require.NoError(t, err)

	{
		referredCount, err := store.TotalReferredCount(ctx, createdUser.UserID)
		require.NoError(t, err)
		require.Equal(t, 1, referredCount)
	}

	{
		availableCredits, err := store.AvailableCredits(ctx, *referrerID, time.Now().UTC())
		require.NoError(t, err)
		var sum int
		for i := range availableCredits {
			sum += availableCredits[i].CreditsEarnedInCents - availableCredits[i].CreditsUsedInCents
		}

		require.Equal(t, userCredit.CreditsEarnedInCents, sum)
	}

}
