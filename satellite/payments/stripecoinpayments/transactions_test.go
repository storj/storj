// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"math/big"
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestTransactionsDB(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		transactions := db.CoinpaymentsTransactions()

		t.Run("insert", func(t *testing.T) {
			amount, received := new(big.Float).SetPrec(1000), new(big.Float).SetPrec(1000)

			amount, ok := amount.SetString("2.0000000000000000005")
			require.True(t, ok)
			received, ok = received.SetString("1.0000000000000000003")
			require.True(t, ok)

			createTx := stripecoinpayments.Transaction{
				ID:        "testID",
				AccountID: uuid.UUID{1, 2, 3},
				Address:   "testAddress",
				Amount:    *amount,
				Received:  *received,
				Status:    coinpayments.StatusReceived,
				Key:       "testKey",
			}

			tx, err := transactions.Insert(ctx, createTx)
			require.NoError(t, err)
			require.NotNil(t, tx)

			assert.Equal(t, createTx.ID, tx.ID)
			assert.Equal(t, createTx.AccountID, tx.AccountID)
			assert.Equal(t, createTx.Address, tx.Address)
			assert.Equal(t, createTx.Amount, tx.Amount)
			assert.Equal(t, createTx.Received, tx.Received)
			assert.Equal(t, createTx.Status, tx.Status)
			assert.Equal(t, createTx.Key, tx.Key)
			assert.False(t, tx.CreatedAt.IsZero())
		})
	})
}
