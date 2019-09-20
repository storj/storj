// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storagenode"
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

func TestNodeInfoUpdated(t *testing.T) {
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

		node.Contact.Service.UpdateSelf(&newCapacity)

		node.Contact.Chore.Loop.TriggerWait()

		newInfo, err := satellite.Overlay.Service.Get(ctx, node.ID())
		require.NoError(t, err)

		firstUptime := oldInfo.Reputation.LastContactSuccess
		secondUptime := newInfo.Reputation.LastContactSuccess
		require.True(t, secondUptime.After(firstUptime))

		require.Equal(t, newCapacity, newInfo.Capacity)
	})
}

func TestRequestInfoEndpointTrustedSatellite(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeDossier := planet.StorageNodes[0].Local()

		// Satellite Trusted
		conn, err := planet.Satellites[0].Transport.DialNode(ctx, &nodeDossier.Node)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		resp, err := pb.NewNodesClient(conn).RequestInfo(ctx, &pb.InfoRequest{})
		require.NotNil(t, resp)
		require.NoError(t, err)
		require.Equal(t, nodeDossier.Type, resp.Type)
		require.Equal(t, &nodeDossier.Operator, resp.Operator)
		require.Equal(t, &nodeDossier.Capacity, resp.Capacity)
		require.Equal(t, nodeDossier.Version.GetVersion(), resp.Version.GetVersion())
	})
}

func TestRequestInfoEndpointUntrustedSatellite(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Storage.WhitelistedSatellites = nil
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeDossier := planet.StorageNodes[0].Local()

		// Satellite Untrusted
		conn, err := planet.Satellites[0].Transport.DialNode(ctx, &nodeDossier.Node)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		resp, err := pb.NewNodesClient(conn).RequestInfo(ctx, &pb.InfoRequest{})
		require.Nil(t, resp)
		require.Error(t, err)
		require.Equal(t, status.Errorf(codes.PermissionDenied, "untrusted peer %v", planet.Satellites[0].Local().Id), err)
	})
}
