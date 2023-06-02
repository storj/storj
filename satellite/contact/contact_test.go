// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact_test

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode"
)

func TestSatelliteContactEndpoint(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeInfo := planet.StorageNodes[0].Contact.Service.Local()
		ident := planet.StorageNodes[0].Identity

		peer := rpcpeer.Peer{
			Addr: &net.TCPAddr{
				IP:   net.ParseIP(nodeInfo.Address),
				Port: 5,
			},
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
			},
		}
		peerCtx := rpcpeer.NewContext(ctx, &peer)
		resp, err := planet.Satellites[0].Contact.Endpoint.CheckIn(peerCtx, &pb.CheckInRequest{
			Address:       nodeInfo.Address,
			Version:       &nodeInfo.Version,
			Capacity:      &nodeInfo.Capacity,
			Operator:      &nodeInfo.Operator,
			DebounceLimit: 3,
			Features:      0xf,
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.PingNodeSuccess)
		require.True(t, resp.PingNodeSuccessQuic)

		peerID, err := planet.Satellites[0].DB.PeerIdentities().Get(ctx, nodeInfo.ID)
		require.NoError(t, err)
		require.Equal(t, ident.PeerIdentity(), peerID)

		node, err := planet.Satellites[0].DB.OverlayCache().Get(ctx, nodeInfo.ID)
		require.NoError(t, err)
		require.Equal(t, node.Address.Address, nodeInfo.Address)
		require.Equal(t, node.Address.DebounceLimit, int32(3))
		require.Equal(t, node.Address.Features, uint64(0xf))
	})
}

func TestSatelliteContactEndpoint_QUIC_Unreachable(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Server.DisableQUIC = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeInfo := planet.StorageNodes[0].Contact.Service.Local()
		ident := planet.StorageNodes[0].Identity

		peer := rpcpeer.Peer{
			Addr: &net.TCPAddr{
				IP:   net.ParseIP(nodeInfo.Address),
				Port: 5,
			},
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
			},
		}
		peerCtx := rpcpeer.NewContext(ctx, &peer)
		resp, err := planet.Satellites[0].Contact.Endpoint.CheckIn(peerCtx, &pb.CheckInRequest{
			Address:  nodeInfo.Address,
			Version:  &nodeInfo.Version,
			Capacity: &nodeInfo.Capacity,
			Operator: &nodeInfo.Operator,
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.PingNodeSuccess)
		require.False(t, resp.PingNodeSuccessQuic)

		peerID, err := planet.Satellites[0].DB.PeerIdentities().Get(ctx, nodeInfo.ID)
		require.NoError(t, err)
		require.Equal(t, ident.PeerIdentity(), peerID)
	})
}

func TestSatellitePingBack_Failure(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		pingNodeSuccess, pingNodeSuccessQUIC, pingErrorMessage, err := planet.Satellites[0].Contact.Service.PingBack(ctx, storj.NodeURL{})

		require.NoError(t, err)
		require.NotEmpty(t, pingErrorMessage)
		require.False(t, pingNodeSuccess)
		require.False(t, pingNodeSuccessQUIC)
	})
}

func TestSatellitePingMeEndpoint(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeInfo := planet.StorageNodes[0].Contact.Service.Local()
		ident := planet.StorageNodes[0].Identity

		peer := rpcpeer.Peer{
			Addr: &net.TCPAddr{
				IP:   net.ParseIP(nodeInfo.Address),
				Port: 5,
			},
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
			},
		}
		peerCtx := rpcpeer.NewContext(ctx, &peer)
		resp, err := planet.Satellites[0].Contact.Endpoint.PingMe(peerCtx, &pb.PingMeRequest{
			Address:   nodeInfo.Address,
			Transport: pb.NodeTransport_TCP_TLS_RPC,
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestSatellitePingMeEndpoint_QUIC(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeInfo := planet.StorageNodes[0].Contact.Service.Local()
		ident := planet.StorageNodes[0].Identity

		peer := rpcpeer.Peer{
			Addr: &net.TCPAddr{
				IP:   net.ParseIP(nodeInfo.Address),
				Port: 5,
			},
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
			},
		}
		peerCtx := rpcpeer.NewContext(ctx, &peer)
		resp, err := planet.Satellites[0].Contact.Endpoint.PingMe(peerCtx, &pb.PingMeRequest{
			Address:   nodeInfo.Address,
			Transport: pb.NodeTransport_QUIC_RPC,
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestSatellitePingMe_Failure(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		resp, err := planet.Satellites[0].Contact.Endpoint.PingMe(ctx, &pb.PingMeRequest{})

		require.NotNil(t, err)
		require.Equal(t, rpcstatus.Code(err), rpcstatus.Unknown)
		require.Nil(t, resp)
	})
}
