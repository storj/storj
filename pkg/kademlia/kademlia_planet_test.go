// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestFetchPeerIdentity(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		peerID, err := planet.StorageNodes[0].Kademlia.Service.FetchPeerIdentity(ctx, sat.ID())
		require.NoError(t, err)
		require.Equal(t, sat.ID(), peerID.ID)
		require.True(t, sat.Identity.Leaf.Equal(peerID.Leaf))
		require.True(t, sat.Identity.CA.Equal(peerID.CA))
	})
}

func TestRequestInfo(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		id, info, err := planet.Satellites[0].Kademlia.Service.FetchInfo(ctx, node.Local().Address)
		require.NoError(t, err)
		require.Equal(t, node.ID(), id.ID)
		require.Equal(t, node.Local().Type, info.GetType())
		require.Equal(t, node.Local().Operator.Email, info.GetOperator().GetEmail())
		require.Equal(t, node.Local().Operator.Wallet, info.GetOperator().GetWallet())
		require.Equal(t, node.Local().Capacity.FreeBandwidth, info.GetCapacity().GetFreeBandwidth())
		require.Equal(t, node.Local().Capacity.FreeDisk, info.GetCapacity().GetFreeDisk())
	})
}
