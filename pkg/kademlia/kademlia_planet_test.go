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
		info, err := planet.Satellites[0].Kademlia.Service.FetchInfo(ctx, node.Local().Address)
		require.NoError(t, err)
		require.Equal(t, node.ID(), info.Id)
		require.Equal(t, node.Local().Type, info.GetType())
		require.Equal(t, node.Local().Metadata.GetEmail(), info.GetMetadata().GetEmail())
		require.Equal(t, node.Local().Metadata.GetWallet(), info.GetMetadata().GetWallet())
		require.Equal(t, node.Local().Restrictions.GetFreeDisk(), info.GetRestrictions().GetFreeDisk())
		require.Equal(t, node.Local().Restrictions.GetFreeBandwidth(), info.GetRestrictions().GetFreeBandwidth())
	})
}
