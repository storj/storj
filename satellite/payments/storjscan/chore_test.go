// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/storjscan/blockchaintest"
	"storj.io/storj/satellite/payments/storjscan/storjscantest"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestChore(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		logger := zaptest.NewLogger(t)
		now := time.Now().Round(time.Second).UTC()

		const confirmations = 12

		var pmnts []storjscan.Payment
		var cachedPayments []storjscan.CachedPayment

		latestBlock := storjscan.Header{
			Hash:      blockchaintest.NewHash(),
			Number:    0,
			Timestamp: now,
		}

		addPayments := func(count int) {
			l := len(pmnts)
			for i := l; i < l+count; i++ {
				payment := storjscan.Payment{
					From:        blockchaintest.NewAddress(),
					To:          blockchaintest.NewAddress(),
					TokenValue:  currency.AmountFromBaseUnits(int64(i)*100000000, currency.StorjToken),
					USDValue:    currency.AmountFromBaseUnits(int64(i)*1100000, currency.USDollarsMicro),
					BlockHash:   blockchaintest.NewHash(),
					BlockNumber: int64(i),
					Transaction: blockchaintest.NewHash(),
					LogIndex:    i,
					Timestamp:   now.Add(time.Duration(i) * time.Second),
				}
				pmnts = append(pmnts, payment)

				cachedPayments = append(cachedPayments, storjscan.CachedPayment{
					From:        payment.From,
					To:          payment.To,
					TokenValue:  payment.TokenValue,
					USDValue:    payment.USDValue,
					Status:      payments.PaymentStatusPending,
					BlockHash:   payment.BlockHash,
					BlockNumber: payment.BlockNumber,
					Transaction: payment.Transaction,
					LogIndex:    payment.LogIndex,
					Timestamp:   payment.Timestamp,
				})
			}

			latestBlock = storjscan.Header{
				Hash:      pmnts[len(pmnts)-1].BlockHash,
				Number:    pmnts[len(pmnts)-1].BlockNumber,
				Timestamp: pmnts[len(pmnts)-1].Timestamp,
			}
			for i := 0; i < len(cachedPayments); i++ {
				if latestBlock.Number-cachedPayments[i].BlockNumber >= confirmations {
					cachedPayments[i].Status = payments.PaymentStatusConfirmed
				} else {
					cachedPayments[i].Status = payments.PaymentStatusPending
				}
			}
		}

		var reqCounter int

		const (
			identifier = "eu"
			secret     = "secret"
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			reqCounter++

			if err = storjscantest.CheckAuth(r, identifier, secret); err != nil {
				storjscantest.ServeJSONError(t, w, http.StatusUnauthorized, err)
				return
			}

			var from int64
			if s := r.URL.Query().Get("from"); s != "" {
				from, err = strconv.ParseInt(s, 10, 64)
				if err != nil {
					storjscantest.ServeJSONError(t, w, http.StatusBadRequest, errs.New("from parameter is missing"))
					return
				}
			}

			storjscantest.ServePayments(t, w, from, latestBlock, pmnts)
		}))
		defer server.Close()

		paymentsDB := db.StorjscanPayments()

		client := storjscan.NewClient(server.URL, "eu", "secret")
		chore := storjscan.NewChore(logger, client, paymentsDB, confirmations, time.Second, false)
		ctx.Go(func() error {
			return chore.Run(ctx)
		})
		defer ctx.Check(chore.Close)

		chore.TransactionCycle.Pause()
		chore.TransactionCycle.TriggerWait()
		cachedReqCounter := reqCounter

		addPayments(100)
		chore.TransactionCycle.TriggerWait()

		last, err := paymentsDB.LastBlock(ctx, payments.PaymentStatusPending)
		require.NoError(t, err)
		require.EqualValues(t, 99, last)
		actual, err := paymentsDB.List(ctx)
		require.NoError(t, err)
		require.Equal(t, cachedPayments, actual)

		addPayments(100)
		chore.TransactionCycle.TriggerWait()

		last, err = paymentsDB.LastBlock(ctx, payments.PaymentStatusPending)
		require.NoError(t, err)
		require.EqualValues(t, 199, last)
		actual, err = paymentsDB.List(ctx)
		require.NoError(t, err)
		require.Equal(t, cachedPayments, actual)

		require.Equal(t, reqCounter, cachedReqCounter+2)
	})
}
