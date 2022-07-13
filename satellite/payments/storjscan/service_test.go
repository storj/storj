// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan_test

import (
	"testing"
	"time"

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
		now := time.Now().Truncate(time.Second)

		wallet1 := blockchaintest.NewAddress()
		wallet2 := blockchaintest.NewAddress()

		walletPayments := []payments.WalletPayment{
			{
				From:        blockchaintest.NewAddress(),
				To:          wallet1,
				TokenValue:  monetary.AmountFromBaseUnits(100, monetary.StorjToken),
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
			expected := []payments.WalletPayment{walletPayments[0], walletPayments[1], walletPayments[3]}

			actual, err := service.Payments(ctx, wallet1, 5, 0)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
		t.Run("wallet 1 from offset", func(t *testing.T) {
			expected := []payments.WalletPayment{walletPayments[1], walletPayments[3]}

			actual, err := service.Payments(ctx, wallet1, 5, 1)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
		t.Run("wallet 1 with limit", func(t *testing.T) {
			expected := []payments.WalletPayment{walletPayments[0], walletPayments[1]}

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
