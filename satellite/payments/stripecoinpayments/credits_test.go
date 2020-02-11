// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestCreditsRepository(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		creditsRepo := db.StripeCoinPayments().Credits()
		userID := testrand.UUID()
		credit := payments.Credit{
			UserID:        userID,
			Amount:        10,
			TransactionID: "transactionID",
		}

		spending := stripecoinpayments.CreditsSpending{
			ProjectID: testrand.UUID(),
			UserID:    userID,
			Amount:    5,
			Status:    stripecoinpayments.CreditsSpendingStatusUnapplied,
		}

		t.Run("insert", func(t *testing.T) {
			err := creditsRepo.InsertCredit(ctx, credit)
			assert.NoError(t, err)

			credits, err := creditsRepo.ListCredits(ctx, userID)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(credits))
			credit = credits[0]

			err = creditsRepo.InsertCreditsSpending(ctx, spending)
			assert.NoError(t, err)

			spendings, err := creditsRepo.ListCreditsSpendings(ctx, userID)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(spendings))
			spending.ID = spendings[0].ID
		})

		t.Run("get credit by transactionID", func(t *testing.T) {
			crdt, err := creditsRepo.GetCredit(ctx, credit.TransactionID)
			assert.NoError(t, err)
			assert.Equal(t, int64(10), crdt.Amount)
		})

		t.Run("update spending", func(t *testing.T) {
			err := creditsRepo.ApplyCreditsSpending(ctx, spending.ID)
			assert.NoError(t, err)

			spendings, err := creditsRepo.ListCreditsSpendings(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, stripecoinpayments.CreditsSpendingStatusApplied, spendings[0].Status)
			spending = spendings[0]
		})

		t.Run("balance", func(t *testing.T) {
			balance, err := creditsRepo.Balance(ctx, userID)
			assert.NoError(t, err)
			assert.Equal(t, 5, int(balance))
		})
	})
}

func TestCreditsRepositoryList(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		creditsDB := db.StripeCoinPayments().Credits()

		const spendLen = 5

		for i := 0; i < spendLen*2+3; i++ {
			userID, err := uuid.New()
			require.NoError(t, err)
			projectID, err := uuid.New()
			require.NoError(t, err)
			spendingID, err := uuid.New()
			require.NoError(t, err)

			spending := stripecoinpayments.CreditsSpending{
				ID:        *spendingID,
				ProjectID: *projectID,
				UserID:    *userID,
				Amount:    int64(5 + i),
				Status:    0,
			}

			err = creditsDB.InsertCreditsSpending(ctx, spending)
			require.NoError(t, err)
		}

		page, err := creditsDB.ListCreditsSpendingsPaged(ctx, 0, 0, spendLen, time.Now())
		require.NoError(t, err)
		require.Equal(t, spendLen, len(page.Spendings))

		assert.True(t, page.Next)
		assert.Equal(t, int64(5), page.NextOffset)

		page, err = creditsDB.ListCreditsSpendingsPaged(ctx, 0, page.NextOffset, spendLen, time.Now())
		require.NoError(t, err)
		require.Equal(t, spendLen, len(page.Spendings))

		assert.True(t, page.Next)
		assert.Equal(t, int64(10), page.NextOffset)

		page, err = creditsDB.ListCreditsSpendingsPaged(ctx, 0, page.NextOffset, spendLen, time.Now())
		require.NoError(t, err)
		require.Equal(t, 3, len(page.Spendings))

		assert.False(t, page.Next)
		assert.Equal(t, int64(0), page.NextOffset)

		const credLen = 5

		user2ID, err := uuid.New()
		require.NoError(t, err)

		for i := 0; i < credLen*2+3; i++ {
			transactionID := "transID" + strconv.Itoa(i)

			credit := payments.Credit{
				UserID:        *user2ID,
				Amount:        5,
				TransactionID: coinpayments.TransactionID(transactionID),
			}

			err = creditsDB.InsertCredit(ctx, credit)
			require.NoError(t, err)
		}

		page2, err := creditsDB.ListCreditsPaged(ctx, 0, spendLen, time.Now(), *user2ID)
		require.NoError(t, err)
		require.Equal(t, spendLen, len(page2.Credits))

		assert.True(t, page2.Next)
		assert.Equal(t, int64(5), page2.NextOffset)

		page2, err = creditsDB.ListCreditsPaged(ctx, page2.NextOffset, spendLen, time.Now(), *user2ID)
		require.NoError(t, err)
		require.Equal(t, spendLen, len(page2.Credits))

		assert.True(t, page2.Next)
		assert.Equal(t, int64(10), page2.NextOffset)

		page2, err = creditsDB.ListCreditsPaged(ctx, page2.NextOffset, spendLen, time.Now(), *user2ID)
		require.NoError(t, err)
		require.Equal(t, 3, len(page2.Credits))

		assert.False(t, page2.Next)
		assert.Equal(t, int64(0), page2.NextOffset)
	})
}
