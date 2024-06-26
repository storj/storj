// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	blockchain2 "storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/testsuite/storjscan/storjscantest"
	"storj.io/storjscan/blockchain"
	"storj.io/storjscan/private/testeth/testtoken"
)

func TestChore(t *testing.T) {
	storjscantest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, stack *storjscantest.Stack) {
		receiver, _ := blockchain.AddressFromHex("0x27e3d303B0B70B1b17f14525b48Ae7c45D34666f")
		err := stack.App.Wallets.Service.Register(ctx, "eu", map[blockchain.Address]string{
			receiver: "test",
		})
		require.NoError(t, err)

		sat := planet.Satellites[0]
		sat.Core.Payments.StorjscanChore.TransactionCycle.Pause()

		// claim wallet
		_, err = sat.API.Payments.StorjscanClient.ClaimNewEthAddress(ctx)
		require.NoError(t, err)

		client := stack.Network.Dial()
		defer client.Close()
		accs := stack.Network.Accounts()

		tk, err := testtoken.NewTestToken(blockchain.Address(stack.Token), client)
		require.NoError(t, err)

		opts := stack.Network.TransactOptions(ctx, accs[0], 1)
		tx, err := tk.Transfer(opts, receiver, big.NewInt(10000))
		require.NoError(t, err)
		rcpt, err := stack.Network.WaitForTx(ctx, tx.Hash())
		require.NoError(t, err)

		block, err := client.BlockByNumber(ctx, rcpt.BlockNumber)
		require.NoError(t, err)
		blockTime := time.Unix(int64(block.Time()), 0)

		sat.Core.Payments.StorjscanChore.TransactionCycle.TriggerWait()

		pmnts, err := sat.API.Payments.StorjscanService.Payments(ctx, blockchain2.Address(receiver), 1, 0)
		require.NoError(t, err)
		require.Len(t, pmnts, 1)

		expected := payments.WalletPayment{
			From:        blockchain2.Address(accs[0].Address),
			To:          blockchain2.Address(receiver),
			TokenValue:  currency.AmountFromBaseUnits(10000, currency.StorjToken),
			USDValue:    currency.AmountFromBaseUnits(100, currency.USDollarsMicro),
			Status:      payments.PaymentStatusPending,
			BlockHash:   blockchain2.Hash(block.Hash()),
			BlockNumber: block.Number().Int64(),
			Transaction: blockchain2.Hash(rcpt.TxHash),
			LogIndex:    0,
			Timestamp:   blockTime.UTC(),
		}
		require.Equal(t, expected, pmnts[0])
	})
}
