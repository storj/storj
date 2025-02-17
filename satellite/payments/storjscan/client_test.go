// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/storjscan/blockchaintest"
	"storj.io/storj/satellite/payments/storjscan/storjscantest"
)

func TestClientMocked(t *testing.T) {
	ctx := testcontext.New(t)
	now := time.Now().Round(time.Second).UTC()
	chainIds := []int64{1337, 5}

	var payments []storjscan.Payment
	for i := 0; i < 100; i++ {
		chainId := chainIds[i%len(chainIds)]
		payments = append(payments, storjscan.Payment{
			ChainID:     chainId,
			From:        blockchaintest.NewAddress(),
			To:          blockchaintest.NewAddress(),
			TokenValue:  currency.AmountFromBaseUnits(int64(i)*100000000, currency.StorjToken),
			USDValue:    currency.AmountFromBaseUnits(int64(i)*1100000, currency.USDollarsMicro),
			BlockHash:   blockchaintest.NewHash(),
			BlockNumber: int64(i),
			Transaction: blockchaintest.NewHash(),
			LogIndex:    i,
			Timestamp:   now.Add(time.Duration(i) * time.Second),
		})
	}
	latestBlocks := []storjscan.Header{
		{
			ChainID:   payments[len(payments)-1].ChainID,
			Hash:      payments[len(payments)-1].BlockHash,
			Number:    payments[len(payments)-1].BlockNumber,
			Timestamp: payments[len(payments)-1].Timestamp,
		},
		{
			ChainID:   payments[len(payments)-2].ChainID,
			Hash:      payments[len(payments)-2].BlockHash,
			Number:    payments[len(payments)-2].BlockNumber,
			Timestamp: payments[len(payments)-2].Timestamp,
		},
	}

	const (
		identifier = "eu"
		secret     = "secret"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := storjscantest.CheckAuth(r, identifier, secret); err != nil {
			storjscantest.ServeJSONError(t, w, http.StatusUnauthorized, err)
			return
		}

		from := make(map[int64]int64)

		for _, chainID := range chainIds {
			// By default, from should scan all chains from block 0
			from[chainID] = 0
			// If from parameter is set for a chain, use it
			if s := r.URL.Query().Get(strconv.FormatInt(chainID, 10)); s != "" {
				block, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					// If from parameter is invalid, continue to the next chain and just scan from block 0
					continue
				}
				from[chainID] = block
			}
		}

		storjscantest.ServePayments(t, w, from, latestBlocks, payments)
	}))
	defer server.Close()

	client := storjscan.NewClient(server.URL, identifier, secret)

	t.Run("all payments from 0", func(t *testing.T) {
		actual, err := client.AllPayments(ctx, nil)
		require.NoError(t, err)
		require.Equal(t, latestBlocks, actual.LatestBlocks)
		require.Equal(t, len(payments), len(actual.Payments))
		sort.Slice(actual.Payments, func(i, j int) bool { return actual.Payments[i].BlockNumber < actual.Payments[j].BlockNumber })
		require.Equal(t, payments, actual.Payments)
	})
	t.Run("all payments from 50", func(t *testing.T) {
		actual, err := client.AllPayments(ctx, map[int64]int64{chainIds[0]: 50, chainIds[1]: 50})
		require.NoError(t, err)
		require.Equal(t, latestBlocks, actual.LatestBlocks)
		require.Equal(t, 50, len(actual.Payments))
		sort.Slice(actual.Payments, func(i, j int) bool { return actual.Payments[i].BlockNumber < actual.Payments[j].BlockNumber })
		require.Equal(t, payments[50:], actual.Payments)
	})
	t.Run("all payments different start per chain ID", func(t *testing.T) {
		actual, err := client.AllPayments(ctx, map[int64]int64{chainIds[0]: 50, chainIds[1]: 0})
		require.NoError(t, err)
		require.Equal(t, latestBlocks, actual.LatestBlocks)
		require.Equal(t, 75, len(actual.Payments))
		require.Equal(t, 50, paymentsFromChain(actual.Payments, chainIds[1]))
		require.Equal(t, 25, paymentsFromChain(actual.Payments, chainIds[0]))
	})
}

func TestClientMockedUnauthorized(t *testing.T) {
	ctx := testcontext.New(t)

	const (
		identifier = "eu"
		secret     = "secret"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := storjscantest.CheckAuth(r, identifier, secret); err != nil {
			storjscantest.ServeJSONError(t, w, http.StatusUnauthorized, err)
			return
		}
	}))
	defer server.Close()

	t.Run("empty credentials", func(t *testing.T) {
		client := storjscan.NewClient(server.URL, "", "")
		_, err := client.AllPayments(ctx, nil)
		require.Error(t, err)
		require.True(t, storjscan.ClientErrUnauthorized.Has(err))
		require.Equal(t, "identifier is invalid", errors.Unwrap(err).Error())
	})

	t.Run("invalid identifier", func(t *testing.T) {
		client := storjscan.NewClient(server.URL, "invalid", "secret")
		_, err := client.AllPayments(ctx, nil)
		require.Error(t, err)
		require.True(t, storjscan.ClientErrUnauthorized.Has(err))
		require.Equal(t, "identifier is invalid", errors.Unwrap(err).Error())
	})

	t.Run("invalid secret", func(t *testing.T) {
		client := storjscan.NewClient(server.URL, "eu", "invalid")
		_, err := client.AllPayments(ctx, nil)
		require.Error(t, err)
		require.True(t, storjscan.ClientErrUnauthorized.Has(err))
		require.Equal(t, "secret is invalid", errors.Unwrap(err).Error())
	})
}

func paymentsFromChain(payments []storjscan.Payment, chainID int64) int {
	var count int
	for _, payment := range payments {
		if payment.ChainID == chainID {
			count++
		}
	}
	return count
}
