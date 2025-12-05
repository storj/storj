// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscantest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/testsuite/storjscan/storjscantest"
	"storj.io/storjscan/blockchain"
)

func TestRun(t *testing.T) {
	storjscantest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, stack *storjscantest.Stack) {
		client := stack.Network.Dial()
		block, err := client.BlockNumber(ctx)
		require.NoError(t, err)
		require.EqualValues(t, 1, block)

		err = stack.App.API.Server.LogRoutes()
		require.NoError(t, err)

		pmnts, err := stack.App.Tokens.Service.Payments(ctx, blockchain.Address{}, 0)
		require.NoError(t, err)
		require.Len(t, pmnts, 0)

		// TODO: add satellite whoami test call
	})
}
