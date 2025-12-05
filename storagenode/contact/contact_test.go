// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact_test

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode/contact"
)

func TestStoragenodeContactEndpoint(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		pingStats := planet.StorageNodes[0].Contact.PingStats

		conn, err := planet.Satellites[0].Dialer.DialNodeURL(ctx, planet.StorageNodes[0].NodeURL())
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		resp, err := pb.NewDRPCContactClient(conn).PingNode(ctx, &pb.ContactPingRequest{})
		require.NotNil(t, resp)
		require.NoError(t, err)

		firstPing := pingStats.WhenLastPinged()

		time.Sleep(time.Second) // HACKFIX: windows has large time granularity

		resp, err = pb.NewDRPCContactClient(conn).PingNode(ctx, &pb.ContactPingRequest{})
		require.NotNil(t, resp)
		require.NoError(t, err)

		secondPing := pingStats.WhenLastPinged()

		require.True(t, secondPing.After(firstPing))
	})
}

func TestNodeInfoUpdated(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.NodeCheckInWaitPeriod = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		node := planet.StorageNodes[0]

		node.Contact.Chore.Pause(ctx)
		oldInfo, err := satellite.Overlay.Service.Get(ctx, node.ID())
		require.NoError(t, err)
		oldCapacity := oldInfo.Capacity

		newCapacity := pb.NodeCapacity{
			FreeDisk: 0,
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

func TestServicePingSatellites(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.NodeCheckInWaitPeriod = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		node.Contact.Chore.Pause(ctx)

		newCapacity := pb.NodeCapacity{
			FreeDisk: 0,
		}
		for _, satellite := range planet.Satellites {
			info, err := satellite.Overlay.Service.Get(ctx, node.ID())
			require.NoError(t, err)
			require.NotEqual(t, newCapacity, info.Capacity)
		}

		node.Contact.Service.UpdateSelf(&newCapacity)
		err := node.Contact.Service.PingSatellites(ctx, 10*time.Second, 15*time.Second)
		require.NoError(t, err)

		for _, satellite := range planet.Satellites {
			info, err := satellite.Overlay.Service.Get(ctx, node.ID())
			require.NoError(t, err)
			require.Equal(t, newCapacity, info.Capacity)
		}
	})
}

func TestEndpointPingNode_UnTrust(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		node.Contact.Chore.Pause(ctx)

		// make sure a trusted satellite is able to ping node
		info, err := planet.Satellites[0].Overlay.Service.Get(ctx, node.ID())
		require.NoError(t, err)
		require.Equal(t, node.ID(), info.Id)

		// an untrusted peer shouldn't be able to ping node successfully
		ident, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)

		state := tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
		}
		peerCtx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			Addr:  node.Server.Addr(),
			State: state,
		})
		_, err = node.Contact.Endpoint.PingNode(peerCtx, &pb.ContactPingRequest{})
		require.Error(t, err)
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

func TestServiceRequestPingMeQUIC(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		node.Contact.Chore.Pause(ctx)

		quicStats, err := node.Contact.Service.RequestPingMeQUIC(ctx)
		require.NoError(t, err)
		require.Equal(t, contact.NetworkStatusOk, quicStats.Status())
	})
}
