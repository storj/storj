// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testplanet_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode"
)

// TestRPCBuild prints a statement so that in test output you can know whether
// the code was compiled with dprc or grpc
func TestRPCBuild(t *testing.T) {
	require.False(t, rpc.IsDRPC == rpc.IsGRPC)

	var rpcType string
	if rpc.IsDRPC {
		rpcType = "Compiled with DRPC"
	} else if rpc.IsGRPC {
		rpcType = "Compiled with GRPC"
	}
	require.NotEqual(t, rpcType, "")

	t.Log(rpcType)
}

func TestDialNode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 0, StorageNodeCount: 2, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		whitelistPath, err := planet.WriteWhitelist(storj.LatestIDVersion())
		require.NoError(t, err)

		unsignedIdent, err := testidentity.PregeneratedIdentity(0, storj.LatestIDVersion())
		require.NoError(t, err)

		signedIdent, err := testidentity.PregeneratedSignedIdentity(0, storj.LatestIDVersion())
		require.NoError(t, err)

		tlsOptions, err := tlsopts.NewOptions(signedIdent, tlsopts.Config{
			UsePeerCAWhitelist:  true,
			PeerCAWhitelistPath: whitelistPath,
			PeerIDVersions:      "*",
		}, nil)
		require.NoError(t, err)

		dialer := rpc.NewDefaultDialer(tlsOptions)

		unsignedClientOpts, err := tlsopts.NewOptions(unsignedIdent, tlsopts.Config{
			PeerIDVersions: "*",
		}, nil)
		require.NoError(t, err)

		unsignedDialer := rpc.NewDefaultDialer(unsignedClientOpts)

		t.Run("DialNode with invalid targets", func(t *testing.T) {
			targets := []*pb.Node{
				{
					Id:      storj.NodeID{},
					Address: nil,
				},
				{
					Id: storj.NodeID{},
					Address: &pb.NodeAddress{
						Transport: pb.NodeTransport_TCP_TLS_GRPC,
					},
				},
				{
					Id: storj.NodeID{123},
					Address: &pb.NodeAddress{
						Transport: pb.NodeTransport_TCP_TLS_GRPC,
						Address:   "127.0.0.1:100",
					},
				},
				{
					Id: storj.NodeID{},
					Address: &pb.NodeAddress{
						Transport: pb.NodeTransport_TCP_TLS_GRPC,
						Address:   planet.StorageNodes[1].Addr(),
					},
				},
			}

			for _, target := range targets {
				tag := fmt.Sprintf("%+v", target)

				timedCtx, cancel := context.WithTimeout(ctx, time.Second)
				conn, err := dialer.DialNode(timedCtx, target)
				cancel()
				assert.Error(t, err, tag)
				assert.Nil(t, conn, tag)
			}
		})

		t.Run("DialNode with valid signed target", func(t *testing.T) {
			target := &pb.Node{
				Id: planet.StorageNodes[1].ID(),
				Address: &pb.NodeAddress{
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
					Address:   planet.StorageNodes[1].Addr(),
				},
			}

			timedCtx, cancel := context.WithTimeout(ctx, time.Second)
			conn, err := dialer.DialNode(timedCtx, target)
			cancel()

			assert.NoError(t, err)
			require.NotNil(t, conn)

			assert.NoError(t, conn.Close())
		})

		t.Run("DialNode with unsigned identity", func(t *testing.T) {
			target := &pb.Node{
				Id: planet.StorageNodes[1].ID(),
				Address: &pb.NodeAddress{
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
					Address:   planet.StorageNodes[1].Addr(),
				},
			}

			timedCtx, cancel := context.WithTimeout(ctx, time.Second)
			conn, err := unsignedDialer.DialNode(timedCtx, target)
			cancel()

			assert.NotNil(t, conn)
			require.NoError(t, err)
			assert.NoError(t, conn.Close())
		})

		t.Run("DialAddress with unsigned identity", func(t *testing.T) {
			timedCtx, cancel := context.WithTimeout(ctx, time.Second)
			conn, err := unsignedDialer.DialAddressInsecure(timedCtx, planet.StorageNodes[1].Addr())
			cancel()

			assert.NotNil(t, conn)
			require.NoError(t, err)
			assert.NoError(t, conn.Close())
		})

		t.Run("DialAddress with valid address", func(t *testing.T) {
			timedCtx, cancel := context.WithTimeout(ctx, time.Second)
			conn, err := dialer.DialAddressInsecure(timedCtx, planet.StorageNodes[1].Addr())
			cancel()

			assert.NoError(t, err)
			require.NotNil(t, conn)
			assert.NoError(t, conn.Close())
		})
	})
}

func TestDialNode_BadServerCertificate(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 0, StorageNodeCount: 2, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Server.UsePeerCAWhitelist = false
			},
			StorageNode: func(index int, config *storagenode.Config) {
				config.Server.UsePeerCAWhitelist = false
			},
			Identities: func(log *zap.Logger, version storj.IDVersion) *testidentity.Identities {
				return testidentity.NewPregeneratedIdentities(version)
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		whitelistPath, err := planet.WriteWhitelist(storj.LatestIDVersion())
		require.NoError(t, err)

		ident, err := testidentity.PregeneratedSignedIdentity(0, storj.LatestIDVersion())
		require.NoError(t, err)

		tlsOptions, err := tlsopts.NewOptions(ident, tlsopts.Config{
			UsePeerCAWhitelist:  true,
			PeerCAWhitelistPath: whitelistPath,
		}, nil)
		require.NoError(t, err)

		dialer := rpc.NewDefaultDialer(tlsOptions)

		t.Run("DialNode with bad server certificate", func(t *testing.T) {
			target := &pb.Node{
				Id: planet.StorageNodes[1].ID(),
				Address: &pb.NodeAddress{
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
					Address:   planet.StorageNodes[1].Addr(),
				},
			}

			timedCtx, cancel := context.WithTimeout(ctx, time.Second)
			conn, err := dialer.DialNode(timedCtx, target)
			cancel()

			tag := fmt.Sprintf("%+v", target)
			assert.Nil(t, conn, tag)
			require.Error(t, err, tag)
			assert.Contains(t, err.Error(), "not signed by any CA in the whitelist")
		})

		t.Run("DialAddress with bad server certificate", func(t *testing.T) {
			timedCtx, cancel := context.WithTimeout(ctx, time.Second)
			conn, err := dialer.DialAddressID(timedCtx, planet.StorageNodes[1].Addr(), planet.StorageNodes[1].ID())
			cancel()

			assert.Nil(t, conn)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not signed by any CA in the whitelist")
		})
	})
}
