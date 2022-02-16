// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments_test

import (
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v72"
	"github.com/zeebo/errs"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/monetary"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestTransactionsDB(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		transactions := db.StripeCoinPayments().Transactions()

		amount, err := monetary.AmountFromString("2.0000000000000000005", monetary.StorjToken)
		require.NoError(t, err)
		received, err := monetary.AmountFromString("1.0000000000000000003", monetary.StorjToken)
		require.NoError(t, err)
		userID := testrand.UUID()

		createTx := stripecoinpayments.Transaction{
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
			createdAt, err := transactions.Insert(ctx, createTx)
			require.NoError(t, err)
			requireSaneTimestamp(t, createdAt)
			txs, err := transactions.ListAccount(ctx, userID)
			require.NoError(t, err)
			require.Len(t, txs, 1)
			compareTransactions(t, createTx, txs[0])
		})

		t.Run("update", func(t *testing.T) {
			received, err := monetary.AmountFromString("6.0000000000000000001", monetary.StorjToken)
			require.NoError(t, err)

			update := stripecoinpayments.TransactionUpdate{
				TransactionID: createTx.ID,
				Status:        coinpayments.StatusReceived,
				Received:      received,
			}

			err = transactions.Update(ctx, []stripecoinpayments.TransactionUpdate{update}, nil)
			require.NoError(t, err)

			page, err := transactions.ListPending(ctx, 0, 1, time.Now())
			require.NoError(t, err)

			require.Len(t, page.Transactions, 1)
			assert.Equal(t, createTx.ID, page.Transactions[0].ID)
			assert.Equal(t, update.Received, page.Transactions[0].Received)
			assert.Equal(t, update.Status, page.Transactions[0].Status)

			err = transactions.Update(ctx,
				[]stripecoinpayments.TransactionUpdate{
					{
						TransactionID: createTx.ID,
						Status:        coinpayments.StatusCompleted,
						Received:      received,
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
			assert.Equal(t, coinpayments.StatusCompleted, page.Transactions[0].Status)
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

func requireSaneTimestamp(t *testing.T, when time.Time) {
	// ensure time value is sane. I apologize to you people of the future when this starts breaking
	require.Truef(t, when.After(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		"%s seems too small to be a valid creation timestamp", when)
	require.Truef(t, when.Before(time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)),
		"%s seems too large to be a valid creation timestamp", when)
}

func TestConcurrentConsume(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		transactions := db.StripeCoinPayments().Transactions()

		const concurrentTries = 30

		amount, err := monetary.AmountFromString("2.0000000000000000005", monetary.StorjToken)
		require.NoError(t, err)
		received, err := monetary.AmountFromString("1.0000000000000000003", monetary.StorjToken)
		require.NoError(t, err)
		userID := testrand.UUID()
		txID := coinpayments.TransactionID("testID")

		_, err = transactions.Insert(ctx,
			stripecoinpayments.Transaction{
				ID:        txID,
				AccountID: userID,
				Address:   "testAddress",
				Amount:    amount,
				Received:  received,
				Status:    coinpayments.StatusPending,
				Key:       "testKey",
				Timeout:   time.Second * 60,
			},
		)
		require.NoError(t, err)

		err = transactions.Update(ctx,
			[]stripecoinpayments.TransactionUpdate{{
				TransactionID: txID,
				Status:        coinpayments.StatusCompleted,
				Received:      received,
			}},
			coinpayments.TransactionIDList{
				txID,
			},
		)
		require.NoError(t, err)

		var errLock sync.Mutex
		var alreadyConsumed []error

		appendError := func(err error) {
			defer errLock.Unlock()
			errLock.Lock()

			alreadyConsumed = append(alreadyConsumed, err)
		}

		var group errs2.Group
		for i := 0; i < concurrentTries; i++ {
			group.Go(func() error {
				err := transactions.Consume(ctx, txID)

				if err == nil {
					return nil
				}
				if errors.Is(err, stripecoinpayments.ErrTransactionConsumed) {
					appendError(err)
					return nil
				}

				return err
			})
		}

		require.NoError(t, errs.Combine(group.Wait()...))
		require.Equal(t, concurrentTries-1, len(alreadyConsumed))
	})
}

func TestTransactionsDBList(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	const (
		limit            = 5
		transactionCount = limit * 4
	)

	// create transactions
	amount, err := monetary.AmountFromString("4.0000000000000000005", monetary.StorjToken)
	require.NoError(t, err)
	received, err := monetary.AmountFromString("5.0000000000000000003", monetary.StorjToken)
	require.NoError(t, err)

	var txs []stripecoinpayments.Transaction
	for i := 0; i < transactionCount; i++ {
		id := base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))
		addr := base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))
		key := base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))

		status := coinpayments.StatusPending
		if i%2 == 0 {
			status = coinpayments.StatusReceived
		}

		createTX := stripecoinpayments.Transaction{
			ID:        coinpayments.TransactionID(id),
			AccountID: uuid.UUID{},
			Address:   addr,
			Amount:    amount,
			Received:  received,
			Status:    status,
			Key:       key,
		}

		txs = append(txs, createTX)
	}

	t.Run("pending transactions", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			for _, tx := range txs {
				_, err := db.StripeCoinPayments().Transactions().Insert(ctx, tx)
				require.NoError(t, err)
			}

			page, err := db.StripeCoinPayments().Transactions().ListPending(ctx, 0, limit, time.Now())
			require.NoError(t, err)

			pendingTXs := page.Transactions

			for page.Next {
				page, err = db.StripeCoinPayments().Transactions().ListPending(ctx, page.NextOffset, limit, time.Now())
				require.NoError(t, err)

				pendingTXs = append(pendingTXs, page.Transactions...)
			}

			require.False(t, page.Next)
			require.Equal(t, transactionCount, len(pendingTXs))

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
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			var updatedTxs []stripecoinpayments.Transaction
			var updates []stripecoinpayments.TransactionUpdate
			var applies coinpayments.TransactionIDList

			for _, tx := range txs {
				_, err := db.StripeCoinPayments().Transactions().Insert(ctx, tx)
				require.NoError(t, err)

				tx.Status = coinpayments.StatusCompleted

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

			page, err := db.StripeCoinPayments().Transactions().ListUnapplied(ctx, 0, limit, time.Now())
			require.NoError(t, err)

			unappliedTXs := page.Transactions

			for page.Next {
				page, err = db.StripeCoinPayments().Transactions().ListUnapplied(ctx, page.NextOffset, limit, time.Now())
				require.NoError(t, err)

				unappliedTXs = append(unappliedTXs, page.Transactions...)
			}

			require.False(t, page.Next)
			require.Equal(t, transactionCount, len(unappliedTXs))

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
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		transactions := db.StripeCoinPayments().Transactions()

		val, err := decimal.NewFromString("4.000000000000005")
		require.NoError(t, err)

		const txID = "tx_id"

		err = transactions.LockRate(ctx, txID, val)
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

func TestTransactions_ApplyTransactionBalance(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		transactions := satellite.API.DB.StripeCoinPayments().Transactions()
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		satellite.Core.Payments.Chore.TransactionCycle.Pause()
		satellite.Core.Payments.Chore.AccountBalanceCycle.Pause()

		// Emulate a deposit through CoinPayments.
		txID := coinpayments.TransactionID("testID")
		storjAmount, err := monetary.AmountFromString("100", monetary.StorjToken)
		require.NoError(t, err)
		storjUSDRate, err := decimal.NewFromString("0.2")
		require.NoError(t, err)

		createTx := stripecoinpayments.Transaction{
			ID:        txID,
			AccountID: userID,
			Address:   "testAddress",
			Amount:    storjAmount,
			Received:  storjAmount,
			Status:    coinpayments.StatusPending,
			Key:       "testKey",
			Timeout:   time.Second * 60,
		}

		tx, err := transactions.Insert(ctx, createTx)
		require.NoError(t, err)
		require.NotNil(t, tx)

		update := stripecoinpayments.TransactionUpdate{
			TransactionID: createTx.ID,
			Status:        coinpayments.StatusReceived,
			Received:      storjAmount,
		}

		err = transactions.Update(ctx, []stripecoinpayments.TransactionUpdate{update}, coinpayments.TransactionIDList{createTx.ID})
		require.NoError(t, err)

		// Check that the CoinPayments transaction is waiting to be applied to the Stripe customer balance.
		page, err := transactions.ListUnapplied(ctx, 0, 1, time.Now())
		require.NoError(t, err)
		require.Len(t, page.Transactions, 1)

		err = transactions.LockRate(ctx, txID, storjUSDRate)
		require.NoError(t, err)

		// Trigger the AccountBalanceCycle. This calls Service.applyTransactionBalance()
		satellite.Core.Payments.Chore.AccountBalanceCycle.TriggerWait()

		cusID, err := satellite.API.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, userID)
		require.NoError(t, err)

		// Check that the CoinPayments deposit is reflected in the Stripe customer balance.
		it := satellite.API.Payments.Stripe.CustomerBalanceTransactions().List(&stripe.CustomerBalanceTransactionListParams{Customer: stripe.String(cusID)})
		require.NoError(t, it.Err())
		require.True(t, it.Next())
		cbt := it.CustomerBalanceTransaction()
		require.EqualValues(t, -2000, cbt.Amount)
		require.EqualValues(t, txID, cbt.Metadata["txID"])
		require.EqualValues(t, "100", cbt.Metadata["storj_amount"])
		require.EqualValues(t, "0.2", cbt.Metadata["storj_usd_rate"])
	})
}
