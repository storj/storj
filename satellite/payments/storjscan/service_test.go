// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/monetary"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/storjscan/blockchaintest"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestServicePayments(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		paymentsDB := db.StorjscanPayments()
		now := time.Now().Truncate(time.Second).UTC()

		wallet1 := blockchaintest.NewAddress()
		wallet2 := blockchaintest.NewAddress()

		walletPayments := []payments.WalletPayment{
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet1,
				TokenValue:  monetary.AmountFromBaseUnits(100, monetary.StorjToken),
				USDValue:    monetary.AmountFromBaseUnits(100, monetary.USDollars),
				Status:      payments.PaymentStatusConfirmed,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: 0,
				Transaction: blockchaintest.NewHash(),
				LogIndex:    0,
				Timestamp:   now,
			},
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet1,
				TokenValue:  monetary.AmountFromBaseUnits(100, monetary.StorjToken),
				USDValue:    monetary.AmountFromBaseUnits(100, monetary.USDollars),
				Status:      payments.PaymentStatusConfirmed,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: 0,
				Transaction: blockchaintest.NewHash(),
				LogIndex:    1,
				Timestamp:   now,
			},
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet2,
				TokenValue:  monetary.AmountFromBaseUnits(100, monetary.StorjToken),
				USDValue:    monetary.AmountFromBaseUnits(100, monetary.USDollars),
				Status:      payments.PaymentStatusConfirmed,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: 0,
				Transaction: blockchaintest.NewHash(),
				LogIndex:    2,
				Timestamp:   now,
			},
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet1,
				TokenValue:  monetary.AmountFromBaseUnits(200, monetary.StorjToken),
				USDValue:    monetary.AmountFromBaseUnits(100, monetary.USDollars),
				Status:      payments.PaymentStatusPending,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: 1,
				Transaction: blockchaintest.NewHash(),
				LogIndex:    0,
				Timestamp:   now.Add(15 * time.Second),
			},
		}

		var cachedPayments []storjscan.CachedPayment
		for _, pmnt := range walletPayments {
			cachedPayments = append(cachedPayments, storjscan.CachedPayment{
				From:        pmnt.From,
				To:          pmnt.To,
				TokenValue:  pmnt.TokenValue,
				USDValue:    pmnt.USDValue,
				Status:      pmnt.Status,
				BlockHash:   pmnt.BlockHash,
				BlockNumber: pmnt.BlockNumber,
				Transaction: pmnt.Transaction,
				LogIndex:    pmnt.LogIndex,
				Timestamp:   pmnt.Timestamp,
			})
		}
		err := paymentsDB.InsertBatch(ctx, cachedPayments)
		require.NoError(t, err)

		service := storjscan.NewService(zaptest.NewLogger(t), db.Wallets(), paymentsDB, nil)

		t.Run("wallet 1", func(t *testing.T) {
			expected := []payments.WalletPayment{walletPayments[3], walletPayments[1], walletPayments[0]}

			actual, err := service.Payments(ctx, wallet1, 5, 0)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
		t.Run("wallet 1 from offset", func(t *testing.T) {
			expected := []payments.WalletPayment{walletPayments[1], walletPayments[0]}

			actual, err := service.Payments(ctx, wallet1, 5, 1)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
		t.Run("wallet 1 with limit", func(t *testing.T) {
			expected := []payments.WalletPayment{walletPayments[3], walletPayments[1]}

			actual, err := service.Payments(ctx, wallet1, 2, 0)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
		t.Run("wallet 2", func(t *testing.T) {
			expected := []payments.WalletPayment{walletPayments[2]}

			actual, err := service.Payments(ctx, wallet2, 1, 0)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
	})
}

func TestListPayments(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		paymentsDB := db.StorjscanPayments()
		now := time.Now().Truncate(time.Second).UTC()

		wallet1 := blockchaintest.NewAddress()
		walletPaymentsBatch1 := []payments.WalletPayment{
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet1,
				TokenValue:  monetary.AmountFromBaseUnits(100, monetary.StorjToken),
				USDValue:    monetary.AmountFromBaseUnits(100, monetary.USDollars),
				Status:      payments.PaymentStatusConfirmed,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: 0,
				Transaction: blockchaintest.NewHash(),
				LogIndex:    1,
				Timestamp:   now,
			},
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet1,
				TokenValue:  monetary.AmountFromBaseUnits(100, monetary.StorjToken),
				USDValue:    monetary.AmountFromBaseUnits(100, monetary.USDollars),
				Status:      payments.PaymentStatusConfirmed,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: 0,
				Transaction: blockchaintest.NewHash(),
				LogIndex:    2,
				Timestamp:   now,
			},
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet1,
				TokenValue:  monetary.AmountFromBaseUnits(100, monetary.StorjToken),
				USDValue:    monetary.AmountFromBaseUnits(100, monetary.USDollars),
				Status:      payments.PaymentStatusConfirmed,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: 0,
				Transaction: blockchaintest.NewHash(),
				LogIndex:    3,
				Timestamp:   now,
			},
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet1,
				TokenValue:  monetary.AmountFromBaseUnits(200, monetary.StorjToken),
				USDValue:    monetary.AmountFromBaseUnits(100, monetary.USDollars),
				Status:      payments.PaymentStatusConfirmed,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: 1,
				Transaction: blockchaintest.NewHash(),
				LogIndex:    0,
				Timestamp:   now.Add(15 * time.Second),
			},
		}

		var cachedPayments1 []storjscan.CachedPayment
		for _, pmnt := range walletPaymentsBatch1 {
			cachedPayments1 = append(cachedPayments1, storjscan.CachedPayment{
				From:        pmnt.From,
				To:          pmnt.To,
				TokenValue:  pmnt.TokenValue,
				USDValue:    pmnt.USDValue,
				Status:      pmnt.Status,
				BlockHash:   pmnt.BlockHash,
				BlockNumber: pmnt.BlockNumber,
				Transaction: pmnt.Transaction,
				LogIndex:    pmnt.LogIndex,
				Timestamp:   pmnt.Timestamp,
			})
		}

		err := paymentsDB.InsertBatch(ctx, cachedPayments1)
		require.NoError(t, err)

		confirmedPayments, err := paymentsDB.ListConfirmed(ctx, 0, 0)
		require.NoError(t, err)
		require.Equal(t, len(cachedPayments1), len(confirmedPayments))
		for _, act := range confirmedPayments {
			for _, exp := range cachedPayments1 {
				if act.BlockHash == exp.BlockHash && act.LogIndex == exp.LogIndex {
					compareTransactions(t, exp, act)
					break
				}
			}
		}

		// these payments should be picked since it's same block number, but has a higher Log index.
		walletPaymentsBatch2 := []payments.WalletPayment{
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet1,
				TokenValue:  monetary.AmountFromBaseUnits(100, monetary.StorjToken),
				USDValue:    monetary.AmountFromBaseUnits(100, monetary.USDollars),
				Status:      payments.PaymentStatusConfirmed,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: 1,
				Transaction: blockchaintest.NewHash(),
				LogIndex:    2,
				Timestamp:   now,
			},
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet1,
				TokenValue:  monetary.AmountFromBaseUnits(200, monetary.StorjToken),
				USDValue:    monetary.AmountFromBaseUnits(100, monetary.USDollars),
				Status:      payments.PaymentStatusConfirmed,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: 1,
				Transaction: blockchaintest.NewHash(),
				LogIndex:    3,
				Timestamp:   now.Add(15 * time.Second),
			},
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet1,
				TokenValue:  monetary.AmountFromBaseUnits(200, monetary.StorjToken),
				USDValue:    monetary.AmountFromBaseUnits(100, monetary.USDollars),
				Status:      payments.PaymentStatusConfirmed,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: 2,
				Transaction: blockchaintest.NewHash(),
				LogIndex:    0,
				Timestamp:   now.Add(15 * time.Second),
			},
		}

		var cachedPayments2 []storjscan.CachedPayment
		for _, pmnt := range walletPaymentsBatch2 {
			cachedPayments2 = append(cachedPayments2, storjscan.CachedPayment{
				From:        pmnt.From,
				To:          pmnt.To,
				TokenValue:  pmnt.TokenValue,
				USDValue:    pmnt.USDValue,
				Status:      pmnt.Status,
				BlockHash:   pmnt.BlockHash,
				BlockNumber: pmnt.BlockNumber,
				Transaction: pmnt.Transaction,
				LogIndex:    pmnt.LogIndex,
				Timestamp:   pmnt.Timestamp,
			})
		}

		err = paymentsDB.InsertBatch(ctx, cachedPayments2)
		require.NoError(t, err)

		confirmedPayments, err = paymentsDB.ListConfirmed(ctx, 1, 1)
		require.NoError(t, err)
		require.Equal(t, len(cachedPayments2), len(confirmedPayments))
		for _, act := range confirmedPayments {
			for _, exp := range cachedPayments2 {
				if act.BlockHash == exp.BlockHash && act.LogIndex == exp.LogIndex {
					compareTransactions(t, exp, act)
					break
				}
			}
		}
	})
}

// compareTransactions is a helper method to compare tx used to create db entry,
// with the tx returned from the db.
func compareTransactions(t *testing.T, exp, act storjscan.CachedPayment) {
	assert.Equal(t, exp.From, act.From)
	assert.Equal(t, exp.To, act.To)
	assert.Equal(t, exp.TokenValue, act.TokenValue)
	assert.Equal(t, exp.USDValue, act.USDValue)
	assert.Equal(t, exp.Status, act.Status)
	assert.Equal(t, exp.BlockHash, act.BlockHash)
	assert.Equal(t, exp.BlockNumber, act.BlockNumber)
	assert.Equal(t, exp.Transaction, act.Transaction)
	assert.Equal(t, exp.LogIndex, act.LogIndex)
	assert.WithinDuration(t, exp.Timestamp, act.Timestamp, time.Microsecond) // database timestamps use microsecond precision
}
