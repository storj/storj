// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

func TestStoragenodeContactEndpoint(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeDossier := planet.StorageNodes[0].Local()
		pingStats := planet.StorageNodes[0].Contact.PingStats

		conn, err := planet.Satellites[0].Dialer.DialNode(ctx, &nodeDossier.Node)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		resp, err := pb.NewDRPCContactClient(conn.Raw()).PingNode(ctx, &pb.ContactPingRequest{})
		require.NotNil(t, resp)
		require.NoError(t, err)

		firstPing := pingStats.WhenLastPinged()

		time.Sleep(time.Second) //HACKFIX: windows has large time granularity

		resp, err = pb.NewDRPCContactClient(conn.Raw()).PingNode(ctx, &pb.ContactPingRequest{})
		require.NotNil(t, resp)
		require.NoError(t, err)

		secondPing := pingStats.WhenLastPinged()

		require.True(t, secondPing.After(firstPing))
	})
}

func TestNodeInfoUpdated(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		node := planet.StorageNodes[0]

		node.Contact.Chore.Pause(ctx)
		oldInfo, err := satellite.Overlay.Service.Get(ctx, node.ID())
		require.NoError(t, err)
		oldCapacity := oldInfo.Capacity

		newCapacity := pb.NodeCapacity{
			FreeBandwidth: 0,
			FreeDisk:      0,
		}
		require.NotEqual(t, oldCapacity, newCapacity)
		node.Contact.Service.UpdateSelf(&newCapacity)

		node.Contact.Chore.TriggerWait(ctx)

		newInfo, err := satellite.Overlay.Service.Get(ctx, node.ID())
		require.NoError(t, err)

		firstUptime := oldInfo.Reputation.LastContactSuccess
		secondUptime := newInfo.Reputation.LastContactSuccess
		require.True(t, secondUptime.After(firstUptime))

		require.Equal(t, newCapacity, newInfo.Capacity)
	})
}

func TestLocalAndUpdateSelf(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		var group errgroup.Group
		group.Go(func() error {
			_ = node.Contact.Service.Local()
			return nil
		})
		node.Contact.Service.UpdateSelf(&pb.NodeCapacity{})
		_ = group.Wait()
	})
}
