// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package billing_test

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/monetary"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestTransactionsDBList(t *testing.T) {
	const (
		limit            = 3
		transactionCount = limit * 4
	)

	// create transactions
	amount, err := monetary.AmountFromString("4", monetary.USDollars)
	require.NoError(t, err)
	userID := testrand.UUID()

	var txs []billing.Transaction
	for i := 0; i < transactionCount; i++ {
		id := base64.StdEncoding.EncodeToString(testrand.Bytes(4 * memory.B))

		txType := billing.Storjscan
		if i%2 == 0 {
			txType = billing.Stripe
		}
		if i%3 == 0 {
			txType = billing.Coinpayments
		}

		createTX := billing.Transaction{
			TXID:        id,
			AccountID:   userID,
			Amount:      amount,
			Description: "credit from storjscan payment",
			TXType:      txType,
			Timestamp:   time.Now(),
			CreatedAt:   time.Now(),
		}

		txs = append(txs, createTX)
	}

	t.Run("add wallet", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			for _, tx := range txs {
				err := db.Billing().Insert(ctx, tx)
				require.NoError(t, err)
			}
			storjscanTXs, err := db.Billing().ListType(ctx, userID, billing.Storjscan)
			require.NoError(t, err)
			require.Equal(t, 4, len(storjscanTXs))
			for _, act := range storjscanTXs {
				for _, exp := range txs {
					if act.TXID == exp.TXID {
						compareTransactions(t, exp, act)
						break
					}
				}
			}

			stripeTXs, err := db.Billing().ListType(ctx, userID, billing.Stripe)
			require.NoError(t, err)
			require.Equal(t, 4, len(stripeTXs))
			for _, act := range stripeTXs {
				for _, exp := range txs {
					if act.TXID == exp.TXID {
						compareTransactions(t, exp, act)
						break
					}
				}
			}

			coinpaymentsTXs, err := db.Billing().ListType(ctx, userID, billing.Coinpayments)
			require.NoError(t, err)
			require.Equal(t, 4, len(coinpaymentsTXs))
			for _, act := range coinpaymentsTXs {
				for _, exp := range txs {
					if act.TXID == exp.TXID {
						compareTransactions(t, exp, act)
						break
					}
				}
			}
		})
	})
}

func TestTransactionsDBBalance(t *testing.T) {
	tenUSD, err := monetary.AmountFromString("10", monetary.USDollars)
	require.NoError(t, err)
	twentyUSD, err := monetary.AmountFromString("20", monetary.USDollars)
	require.NoError(t, err)
	thirtyUSD, err := monetary.AmountFromString("30", monetary.USDollars)
	require.NoError(t, err)
	fortyUSD, err := monetary.AmountFromString("40", monetary.USDollars)
	require.NoError(t, err)
	negativeTwentyUSD, err := monetary.AmountFromString("-20", monetary.USDollars)
	require.NoError(t, err)
	userID := testrand.UUID()

	credit10TX := billing.Transaction{
		TXID:        "billing_txn_10",
		AccountID:   userID,
		Amount:      tenUSD,
		Description: "credit from storjscan payment",
		TXType:      billing.Storjscan,
		Timestamp:   time.Now().Add(time.Second),
		CreatedAt:   time.Now(),
	}

	credit30TX := billing.Transaction{
		TXID:        "billing_txn_30",
		AccountID:   userID,
		Amount:      thirtyUSD,
		Description: "credit from storjscan payment",
		TXType:      billing.Storjscan,
		Timestamp:   time.Now().Add(time.Second * 2),
		CreatedAt:   time.Now(),
	}

	charge20TX := billing.Transaction{
		TXID:        "billing_txn_-20",
		AccountID:   userID,
		Amount:      negativeTwentyUSD,
		Description: "charge for storage and bandwidth",
		TXType:      billing.Stripe,
		Timestamp:   time.Now().Add(time.Second * 3),
		CreatedAt:   time.Now(),
	}

	t.Run("add 10 USD to account", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			err := db.Billing().Insert(ctx, credit10TX)
			require.NoError(t, err)
			txs, err := db.Billing().List(ctx, userID)
			require.NoError(t, err)
			require.Len(t, txs, 1)
			compareTransactions(t, credit10TX, txs[0])
			balance, err := db.Billing().ComputeBalance(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, tenUSD, balance)
		})
	})

	t.Run("add 10 and 30 USD to account", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			err := db.Billing().Insert(ctx, credit10TX)
			require.NoError(t, err)
			err = db.Billing().Insert(ctx, credit30TX)
			require.NoError(t, err)
			txs, err := db.Billing().List(ctx, userID)
			require.NoError(t, err)
			require.Len(t, txs, 2)
			compareTransactions(t, credit30TX, txs[0])
			compareTransactions(t, credit10TX, txs[1])
			balance, err := db.Billing().ComputeBalance(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, fortyUSD, balance)
		})
	})

	t.Run("add 10 USD, add 30 USD, subtract 20 USD", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			err := db.Billing().Insert(ctx, credit10TX)
			require.NoError(t, err)
			err = db.Billing().Insert(ctx, credit30TX)
			require.NoError(t, err)
			err = db.Billing().Insert(ctx, charge20TX)
			require.NoError(t, err)
			txs, err := db.Billing().List(ctx, userID)
			require.NoError(t, err)
			require.Len(t, txs, 3)
			compareTransactions(t, charge20TX, txs[0])
			compareTransactions(t, credit30TX, txs[1])
			compareTransactions(t, credit10TX, txs[2])
			balance, err := db.Billing().ComputeBalance(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, twentyUSD, balance)
		})
	})
}

// compareTransactions is a helper method to compare tx used to create db entry,
// with the tx returned from the db. Method doesn't compare created at field, but
// ensures that is not empty.
func compareTransactions(t *testing.T, exp, act billing.Transaction) {
	assert.Equal(t, exp.TXID, act.TXID)
	assert.Equal(t, exp.AccountID, act.AccountID)
	assert.Equal(t, exp.Amount, act.Amount)
	assert.Equal(t, exp.Description, act.Description)
	assert.Equal(t, exp.TXType, act.TXType)
	assert.WithinDuration(t, exp.Timestamp, act.Timestamp, time.Microsecond) // database timestamps use microsecond precision
	assert.False(t, act.CreatedAt.IsZero())
}
