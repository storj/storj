// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"encoding/base64"
	"math/big"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestTransactionsDB(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		transactions := db.StripeCoinPayments().Transactions()

		amount, ok := new(big.Float).SetPrec(1000).SetString("2.0000000000000000005")
		require.True(t, ok)
		received, ok := new(big.Float).SetPrec(1000).SetString("1.0000000000000000003")
		require.True(t, ok)

		createTx := stripecoinpayments.Transaction{
			ID:        "testID",
			AccountID: uuid.UUID{1, 2, 3},
			Address:   "testAddress",
			Amount:    *amount,
			Received:  *received,
			Status:    coinpayments.StatusReceived,
			Key:       "testKey",
			Timeout:   time.Second * 60,
		}

		t.Run("insert", func(t *testing.T) {
			tx, err := transactions.Insert(ctx, createTx)
			require.NoError(t, err)
			require.NotNil(t, tx)

			compareTransactions(t, createTx, *tx)
		})

		t.Run("update", func(t *testing.T) {
			received, ok := new(big.Float).SetPrec(1000).SetString("6.0000000000000000001")
			require.True(t, ok)

			update := stripecoinpayments.TransactionUpdate{
				TransactionID: createTx.ID,
				Status:        coinpayments.StatusPending,
				Received:      *received,
			}

			err := transactions.Update(ctx, []stripecoinpayments.TransactionUpdate{update}, nil)
			require.NoError(t, err)

			page, err := transactions.ListPending(ctx, 0, 1, time.Now())
			require.NoError(t, err)

			require.NotNil(t, page.Transactions)
			require.Equal(t, 1, len(page.Transactions))
			assert.Equal(t, createTx.ID, page.Transactions[0].ID)
			assert.Equal(t, update.Received, page.Transactions[0].Received)
			assert.Equal(t, update.Status, page.Transactions[0].Status)

			err = transactions.Update(ctx,
				[]stripecoinpayments.TransactionUpdate{
					{
						TransactionID: createTx.ID,
						Status:        coinpayments.StatusReceived,
						Received:      *received,
					},
				},
				coinpayments.TransactionIDList{
					createTx.ID,
				},
			)
			require.NoError(t, err)

			page, err = transactions.ListUnapplied(ctx, 0, 1, time.Now())
			require.NoError(t, err)
			require.NotNil(t, page.Transactions)
			require.Equal(t, 1, len(page.Transactions))

			assert.Equal(t, createTx.ID, page.Transactions[0].ID)
			assert.Equal(t, update.Received, page.Transactions[0].Received)
			assert.Equal(t, coinpayments.StatusReceived, page.Transactions[0].Status)
		})

		t.Run("consume", func(t *testing.T) {
			err := transactions.Consume(ctx, createTx.ID)
			require.NoError(t, err)

			page, err := transactions.ListUnapplied(ctx, 0, 1, time.Now())
			require.NoError(t, err)

			assert.Nil(t, page.Transactions)
			assert.Equal(t, 0, len(page.Transactions))
		})
	})
}

func TestTransactionsDBList(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	const (
		transactionCount = 10
	)

	// create transactions
	amount, ok := new(big.Float).SetPrec(1000).SetString("4.0000000000000000005")
	require.True(t, ok)
	received, ok := new(big.Float).SetPrec(1000).SetString("5.0000000000000000003")
	require.True(t, ok)

	var txs []stripecoinpayments.Transaction
	for i := 0; i < transactionCount; i++ {
		id := base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))
		addr := base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))
		key := base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))

		createTX := stripecoinpayments.Transaction{
			ID:        coinpayments.TransactionID(id),
			AccountID: uuid.UUID{},
			Address:   addr,
			Amount:    *amount,
			Received:  *received,
			Status:    coinpayments.StatusPending,
			Key:       key,
		}

		txs = append(txs, createTX)
	}
	t.Run("pending transactions", func(t *testing.T) {
		satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
			for _, tx := range txs {
				_, err := db.StripeCoinPayments().Transactions().Insert(ctx, tx)
				require.NoError(t, err)
			}

			page, err := db.StripeCoinPayments().Transactions().ListPending(ctx, 0, transactionCount, time.Now())
			require.NoError(t, err)
			require.Equal(t, transactionCount, len(page.Transactions))

			for _, act := range page.Transactions {
				for _, exp := range txs {
					if act.ID == exp.ID {
						compareTransactions(t, exp, act)
					}
				}
			}
		})
	})

	t.Run("unapplied transaction", func(t *testing.T) {
		satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
			var updatedTxs []stripecoinpayments.Transaction
			var updates []stripecoinpayments.TransactionUpdate
			var applies coinpayments.TransactionIDList

			for _, tx := range txs {
				_, err := db.StripeCoinPayments().Transactions().Insert(ctx, tx)
				require.NoError(t, err)

				tx.Status = coinpayments.StatusReceived

				updates = append(updates,
					stripecoinpayments.TransactionUpdate{
						TransactionID: tx.ID,
						Status:        tx.Status,
						Received:      tx.Received,
					},
				)

				applies = append(applies, tx.ID)
				updatedTxs = append(updatedTxs, tx)
			}

			err := db.StripeCoinPayments().Transactions().Update(ctx, updates, applies)
			require.NoError(t, err)

			page, err := db.StripeCoinPayments().Transactions().ListUnapplied(ctx, 0, transactionCount, time.Now())
			require.NoError(t, err)
			require.Equal(t, transactionCount, len(page.Transactions))

			for _, act := range page.Transactions {
				for _, exp := range updatedTxs {
					if act.ID == exp.ID {
						compareTransactions(t, exp, act)
					}
				}
			}
		})
	})
}

func TestTransactionsDBRates(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		transactions := db.StripeCoinPayments().Transactions()

		val, ok := new(big.Float).SetPrec(1000).SetString("4.0000000000000000005")
		require.True(t, ok)

		const txID = "tx_id"

		err := transactions.LockRate(ctx, txID, val)
		require.NoError(t, err)

		rate, err := transactions.GetLockedRate(ctx, txID)
		require.NoError(t, err)

		assert.Equal(t, val, rate)
	})
}

// compareTransactions is a helper method to compare tx used to create db entry,
// with the tx returned from the db. Method doesn't compare created at field, but
// ensures that is not empty.
func compareTransactions(t *testing.T, exp, act stripecoinpayments.Transaction) {
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
