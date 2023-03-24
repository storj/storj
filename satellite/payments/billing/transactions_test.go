// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package billing_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestTransactionsDBList(t *testing.T) {
	const (
		limit            = 3
		transactionCount = limit * 4
	)

	// create transactions
	userID := testrand.UUID()

	var txs []billing.Transaction
	var txStatus billing.TransactionStatus
	var txType billing.TransactionType
	for i := 0; i < transactionCount; i++ {
		txSource := "storjscan"
		txStatus = billing.TransactionStatusCompleted
		txType = billing.TransactionTypeCredit
		if i%2 == 0 {
			txSource = "stripe"
		}
		if i%3 == 0 {
			txSource = "coinpayments"
		}

		address, err := blockchain.BytesToAddress(testrand.Bytes(20))
		require.NoError(t, err)

		metadata, err := json.Marshal(map[string]interface{}{
			"ReferenceID": "some stripe invoice ID",
			"Wallet":      address.Hex(),
		})
		require.NoError(t, err)

		createTX := billing.Transaction{
			UserID:      userID,
			Amount:      currency.AmountFromBaseUnits(4, currency.USDollars),
			Description: "credit from storjscan payment",
			Source:      txSource,
			Status:      txStatus,
			Type:        txType,
			Metadata:    metadata,
			Timestamp:   time.Now(),
			CreatedAt:   time.Now(),
		}

		txs = append(txs, createTX)
	}

	t.Run("insert and list transactions", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			for _, tx := range txs {
				_, err := db.Billing().Insert(ctx, tx)
				require.NoError(t, err)
			}

			actual, err := db.Billing().List(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, len(txs), len(actual))

			// The listing is in descending insertion order so compare
			// accordingly (first listed compared with last inserted, etc.)
			for i, act := range actual {
				exp := txs[len(txs)-i-1]
				compareTransactions(t, exp, act)
			}
		})
	})
}

func TestTransactionsDBBalance(t *testing.T) {
	tenUSD := currency.AmountFromBaseUnits(1000, currency.USDollars)
	tenMicroUSD := currency.AmountFromBaseUnits(10000000, currency.USDollarsMicro)
	twentyMicroUSD := currency.AmountFromBaseUnits(20000000, currency.USDollarsMicro)
	thirtyUSD := currency.AmountFromBaseUnits(3000, currency.USDollars)
	fortyMicroUSD := currency.AmountFromBaseUnits(40000000, currency.USDollarsMicro)
	negativeTwentyUSD := currency.AmountFromBaseUnits(-2000, currency.USDollars)

	userID := testrand.UUID()

	address, err := blockchain.BytesToAddress(testrand.Bytes(20))
	require.NoError(t, err)

	metadata, err := json.Marshal(map[string]interface{}{
		"ReferenceID": "some stripe invoice ID",
		"Wallet":      address.Hex(),
	})
	require.NoError(t, err)

	credit10TX := billing.Transaction{
		UserID:      userID,
		Amount:      tenUSD,
		Description: "credit from storjscan payment",
		Source:      "storjscan",
		Status:      billing.TransactionStatusCompleted,
		Type:        billing.TransactionTypeCredit,
		Metadata:    metadata,
		Timestamp:   time.Now().Add(time.Second),
		CreatedAt:   time.Now(),
	}

	credit30TX := billing.Transaction{
		UserID:      userID,
		Amount:      thirtyUSD,
		Description: "credit from storjscan payment",
		Source:      "storjscan",
		Status:      billing.TransactionStatusCompleted,
		Type:        billing.TransactionTypeCredit,
		Metadata:    metadata,
		Timestamp:   time.Now().Add(time.Second * 2),
		CreatedAt:   time.Now(),
	}

	charge20TX := billing.Transaction{
		UserID:      userID,
		Amount:      negativeTwentyUSD,
		Description: "charge for storage and bandwidth",
		Source:      "storjscan",
		Status:      billing.TransactionStatusCompleted,
		Type:        billing.TransactionTypeDebit,
		Metadata:    metadata,
		Timestamp:   time.Now().Add(time.Second * 3),
		CreatedAt:   time.Now(),
	}

	t.Run("add 10 USD to account", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			_, err := db.Billing().Insert(ctx, credit10TX)
			require.NoError(t, err)
			txs, err := db.Billing().List(ctx, userID)
			require.NoError(t, err)
			require.Len(t, txs, 1)
			compareTransactions(t, credit10TX, txs[0])
			balance, err := db.Billing().GetBalance(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, tenMicroUSD.BaseUnits(), balance.BaseUnits())
		})
	})

	t.Run("add 10 and 30 USD to account", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			_, err := db.Billing().Insert(ctx, credit10TX)
			require.NoError(t, err)
			_, err = db.Billing().Insert(ctx, credit30TX)
			require.NoError(t, err)
			txs, err := db.Billing().List(ctx, userID)
			require.NoError(t, err)
			require.Len(t, txs, 2)
			compareTransactions(t, credit30TX, txs[0])
			compareTransactions(t, credit10TX, txs[1])
			balance, err := db.Billing().GetBalance(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, fortyMicroUSD.BaseUnits(), balance.BaseUnits())
		})
	})

	t.Run("add 10 USD, add 30 USD, subtract 20 USD", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			_, err := db.Billing().Insert(ctx, credit10TX)
			require.NoError(t, err)
			_, err = db.Billing().Insert(ctx, credit30TX)
			require.NoError(t, err)
			_, err = db.Billing().Insert(ctx, charge20TX)
			require.NoError(t, err)
			txs, err := db.Billing().List(ctx, userID)
			require.NoError(t, err)
			require.Len(t, txs, 3)
			compareTransactions(t, charge20TX, txs[0])
			compareTransactions(t, credit30TX, txs[1])
			compareTransactions(t, credit10TX, txs[2])
			balance, err := db.Billing().GetBalance(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, twentyMicroUSD.BaseUnits(), balance.BaseUnits())
		})
	})
}

func TestUpdateTransactions(t *testing.T) {
	tenUSD := currency.AmountFromBaseUnits(1000, currency.USDollars)
	userID := testrand.UUID()
	address, err := blockchain.BytesToAddress(testrand.Bytes(20))
	require.NoError(t, err)

	metadata, err := json.Marshal(map[string]interface{}{
		"ReferenceID": "some stripe invoice ID",
		"Wallet":      address.Hex(),
	})
	require.NoError(t, err)

	credit10TX := billing.Transaction{
		UserID:      userID,
		Amount:      tenUSD,
		Description: "credit from storjscan payment",
		Source:      "storjscan",
		Status:      billing.TransactionStatusCompleted,
		Type:        billing.TransactionTypeCredit,
		Metadata:    metadata,
		Timestamp:   time.Now().Add(time.Second),
		CreatedAt:   time.Now(),
	}

	t.Run("update metadata", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			txIDs, err := db.Billing().Insert(ctx, credit10TX)
			require.NoError(t, err)
			newAddress, err := blockchain.BytesToAddress(testrand.Bytes(20))
			require.NoError(t, err)
			metadata, err := json.Marshal(map[string]interface{}{
				"Wallet": newAddress.Hex(),
			})
			require.NoError(t, err)
			err = db.Billing().UpdateMetadata(ctx, txIDs[0], metadata)
			require.NoError(t, err)
			expMetadata, err := json.Marshal(map[string]interface{}{
				"ReferenceID": "some stripe invoice ID",
				"Wallet":      newAddress.Hex(),
			})
			require.NoError(t, err)
			credit10TX.Metadata = expMetadata
			tx, err := db.Billing().List(ctx, userID)
			require.NoError(t, err)
			compareTransactions(t, credit10TX, tx[0])
		})
	})

	t.Run("update status", func(t *testing.T) {
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			txIDs, err := db.Billing().Insert(ctx, credit10TX)
			require.NoError(t, err)
			err = db.Billing().UpdateStatus(ctx, txIDs[0], billing.TransactionStatusCancelled)
			require.NoError(t, err)
			credit10TX.Status = billing.TransactionStatusCancelled
			tx, err := db.Billing().List(ctx, userID)
			require.NoError(t, err)
			compareTransactions(t, credit10TX, tx[0])
		})
	})
}

func TestUpdateMetadata(t *testing.T) {

}

// compareTransactions is a helper method to compare tx used to create db entry,
// with the tx returned from the db. Method doesn't compare created at field, but
// ensures that is not empty.
func compareTransactions(t *testing.T, exp, act billing.Transaction) {
	assert.Equal(t, exp.UserID, act.UserID)
	assert.Equal(t, currency.AmountFromDecimal(exp.Amount.AsDecimal().Truncate(currency.USDollarsMicro.DecimalPlaces()), currency.USDollarsMicro), act.Amount)
	assert.Equal(t, exp.Description, act.Description)
	assert.Equal(t, exp.Status, act.Status)
	assert.Equal(t, exp.Source, act.Source)
	assert.Equal(t, exp.Type, act.Type)
	var expUpdatedMetadata map[string]string
	var actUpdatedMetadata map[string]string
	err := json.Unmarshal(exp.Metadata, &expUpdatedMetadata)
	require.NoError(t, err)
	err = json.Unmarshal(act.Metadata, &actUpdatedMetadata)
	require.NoError(t, err)
	assert.Equal(t, expUpdatedMetadata["ReferenceID"], actUpdatedMetadata["ReferenceID"])
	assert.Equal(t, expUpdatedMetadata["Wallet"], actUpdatedMetadata["Wallet"])
	assert.WithinDuration(t, exp.Timestamp, act.Timestamp, time.Microsecond) // database timestamps use microsecond precision
	assert.False(t, act.CreatedAt.IsZero())
}
