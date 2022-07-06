// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/testsuite/storjscan/storjscantest"
	"storj.io/storjscan/blockchain"
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
