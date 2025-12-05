// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	blockchain2 "storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/testsuite/storjscan/storjscantest"
	"storj.io/storjscan/blockchain"
	"storj.io/storjscan/private/testeth/testtoken"
)

func TestClientWalletsClaim(t *testing.T) {
	storjscantest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, stack *storjscantest.Stack) {
		expected, _ := blockchain.AddressFromHex("0x27e3d303B0B70B1b17f14525b48Ae7c45D34666f")
		err := stack.App.Wallets.Service.Register(ctx, "eu", map[blockchain.Address]string{
			expected: "test",
		})
		require.NoError(t, err)

		addr, err := planet.Satellites[0].API.Payments.StorjscanClient.ClaimNewEthAddress(ctx)
		require.NoError(t, err)
		require.Equal(t, expected, blockchain.Address(addr))
	})
}

func TestClientPayments(t *testing.T) {
	storjscantest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, stack *storjscantest.Stack) {
		receiver, _ := blockchain.AddressFromHex("0x27e3d303B0B70B1b17f14525b48Ae7c45D34666f")
		err := stack.App.Wallets.Service.Register(ctx, "eu", map[blockchain.Address]string{
			receiver: "test",
		})
		require.NoError(t, err)

		// claim wallet
		_, err = planet.Satellites[0].API.Payments.StorjscanClient.ClaimNewEthAddress(ctx)
		require.NoError(t, err)

		client := stack.Network.Dial()
		defer client.Close()
		accs := stack.Network.Accounts()

		tk, err := testtoken.NewTestToken(blockchain.Address(stack.Token), client)
		require.NoError(t, err)

		opts := stack.Network.TransactOptions(ctx, accs[0], 1)
		tx, err := tk.Transfer(opts, receiver, big.NewInt(100000000))
		require.NoError(t, err)
		rcpt, err := stack.Network.WaitForTx(ctx, tx.Hash())
		require.NoError(t, err)

		block, err := client.BlockByNumber(ctx, rcpt.BlockNumber)
		require.NoError(t, err)
		blockTime := time.Unix(int64(block.Time()), 0)

		// fill price DB
		price := currency.AmountFromBaseUnits(1000000, currency.USDollarsMicro)
		err = stack.App.TokenPrice.Service.SavePrice(ctx, blockTime.Add(-30*time.Second), price)
		require.NoError(t, err)

		pmnts, err := planet.Satellites[0].API.Payments.StorjscanClient.AllPayments(ctx, nil)
		require.NoError(t, err)
		require.Equal(t, block.Number().Int64(), pmnts.LatestBlocks[0].Number)
		require.Len(t, pmnts.Payments, 1)

		expected := storjscan.Payment{
			From:        blockchain2.Address(accs[0].Address),
			To:          blockchain2.Address(receiver),
			TokenValue:  currency.AmountFromBaseUnits(100000000, currency.StorjToken),
			USDValue:    currency.AmountFromBaseUnits(1000000, currency.USDollarsMicro),
			BlockHash:   blockchain2.Hash(block.Hash()),
			BlockNumber: block.Number().Int64(),
			Transaction: blockchain2.Hash(rcpt.TxHash),
			LogIndex:    0,
			Timestamp:   blockTime.UTC(),
		}
		require.Equal(t, expected, pmnts.Payments[0])
	})
}
