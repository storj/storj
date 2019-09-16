// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
)

func TestStoragenodeContactEndpoint(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeDossier := planet.StorageNodes[0].Local()
		pingStats := planet.StorageNodes[0].Contact.PingStats

		conn, err := planet.Satellites[0].Transport.DialNode(ctx, &nodeDossier.Node)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		resp, err := pb.NewContactClient(conn).PingNode(ctx, &pb.ContactPingRequest{})
		require.NotNil(t, resp)
		require.NoError(t, err)

		firstPing, _, _ := pingStats.WhenLastPinged()

		resp, err = pb.NewContactClient(conn).PingNode(ctx, &pb.ContactPingRequest{})
		require.NotNil(t, resp)
		require.NoError(t, err)

		secondPing, _, _ := pingStats.WhenLastPinged()

		require.True(t, secondPing.After(firstPing))
	})
}

func TestContactChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		node := planet.StorageNodes[0]

		node.Contact.Chore.Loop.Pause()

		oldInfo, err := satellite.Overlay.Service.Get(ctx, node.ID())
		require.NoError(t, err)

		oldCapacity := oldInfo.Capacity

		newCapacity := pb.NodeCapacity{
			FreeBandwidth: 0,
			FreeDisk:      0,
		}
		require.NotEqual(t, oldCapacity, newCapacity)

		node.Kademlia.RoutingTable.UpdateSelf(&newCapacity)

		node.Contact.Chore.Loop.TriggerWait()

		newInfo, err := satellite.Overlay.Service.Get(ctx, node.ID())
		require.NoError(t, err)

		firstUptime := oldInfo.Reputation.LastContactSuccess
		secondUptime := newInfo.Reputation.LastContactSuccess
		require.True(t, secondUptime.After(firstUptime))

		require.Equal(t, newCapacity, newInfo.Capacity)
	})
}
