// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/currency"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestTransactionsDB(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		transactions := db.StripeCoinPayments().Transactions()

		amount, err := currency.AmountFromString("2.0000000000000000005", currency.StorjToken)
		require.NoError(t, err)
		received, err := currency.AmountFromString("1.0000000000000000003", currency.StorjToken)
		require.NoError(t, err)
		userID := testrand.UUID()

		createTx := stripe.Transaction{
			ID:        "testID",
			AccountID: userID,
			Address:   "testAddress",
			Amount:    amount,
			Received:  received,
			Status:    coinpayments.StatusPending,
			Key:       "testKey",
			Timeout:   time.Second * 60,
		}

		t.Run("insert", func(t *testing.T) {
			createdAt, err := transactions.TestInsert(ctx, createTx)
			require.NoError(t, err)
			requireSaneTimestamp(t, createdAt)
			txs, err := transactions.ListAccount(ctx, userID)
			require.NoError(t, err)
			require.Len(t, txs, 1)
			compareTransactions(t, createTx, txs[0])
		})
	})
}

func requireSaneTimestamp(t *testing.T, when time.Time) {
	// ensure time value is sane. I apologize to you people of the future when this starts breaking
	require.Truef(t, when.After(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		"%s seems too small to be a valid creation timestamp", when)
	require.Truef(t, when.Before(time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)),
		"%s seems too large to be a valid creation timestamp", when)
}

func TestTransactionsDBList(t *testing.T) {
	const (
		limit            = 5
		transactionCount = limit * 4
	)

	// create transactions
	amount, err := currency.AmountFromString("4.0000000000000000005", currency.StorjToken)
	require.NoError(t, err)
	received, err := currency.AmountFromString("5.0000000000000000003", currency.StorjToken)
	require.NoError(t, err)

	var txs []stripe.Transaction
	for i := 0; i < transactionCount; i++ {
		id := base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))
		addr := base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))
		key := base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))

		createTX := stripe.Transaction{
			ID:        coinpayments.TransactionID(id),
			AccountID: uuid.UUID{},
			Address:   addr,
			Amount:    amount,
			Received:  received,
			Status:    coinpayments.StatusCompleted,
			Key:       key,
		}

		txs = append(txs, createTX)
	}

	t.Run("account", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			for _, tx := range txs {
				_, err := db.StripeCoinPayments().Transactions().TestInsert(ctx, tx)
				require.NoError(t, err)
			}

			accTxs, err := db.StripeCoinPayments().Transactions().ListAccount(ctx, uuid.UUID{})
			require.NoError(t, err)

			for _, act := range accTxs {
				for _, exp := range txs {
					if act.ID == exp.ID {
						compareTransactions(t, exp, act)
					}
				}
			}
		})
	})
}

func TestTransactionsDBRates(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		transactions := db.StripeCoinPayments().Transactions()

		val, err := decimal.NewFromString("4.000000000000005")
		require.NoError(t, err)

		const txID = "tx_id"

		err = transactions.TestLockRate(ctx, txID, val)
		require.NoError(t, err)

		rate, err := transactions.GetLockedRate(ctx, txID)
		require.NoError(t, err)

		assert.Equal(t, val, rate)
	})
}

// compareTransactions is a helper method to compare tx used to create db entry,
// with the tx returned from the db. Method doesn't compare created at field, but
// ensures that is not empty.
func compareTransactions(t *testing.T, exp, act stripe.Transaction) {
	assert.Equal(t, exp.ID, act.ID)
	assert.Equal(t, exp.AccountID, act.AccountID)
	assert.Equal(t, exp.Address, act.Address)
	assert.Equal(t, exp.Amount, act.Amount)
	assert.Equal(t, exp.Received, act.Received)
	assert.Equal(t, exp.Status, act.Status)
	assert.Equal(t, exp.Key, act.Key)
	assert.Equal(t, exp.Timeout, act.Timeout)
	assert.False(t, act.CreatedAt.IsZero())
}
