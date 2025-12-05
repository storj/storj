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
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/rpc/quic"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode"
)

func TestDialNodeURL(t *testing.T) {
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

		tcpDialer := rpc.NewDefaultDialer(tlsOptions)
		quicDialer := rpc.NewDefaultDialer(tlsOptions)
		quicDialer.Connector = quic.NewDefaultConnector(nil)

		unsignedClientOpts, err := tlsopts.NewOptions(unsignedIdent, tlsopts.Config{
			PeerIDVersions: "*",
		}, nil)
		require.NoError(t, err)

		unsignedTCPDialer := rpc.NewDefaultDialer(unsignedClientOpts)
		unsignedQUICDialer := rpc.NewDefaultDialer(unsignedClientOpts)
		unsignedQUICDialer.Connector = quic.NewDefaultConnector(nil)

		test := func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, dialer rpc.Dialer, unsignedDialer rpc.Dialer) {
			t.Run("DialNodeURL with invalid targets", func(t *testing.T) {
				targets := []storj.NodeURL{
					{
						ID:      storj.NodeID{},
						Address: "",
					},
					{
						ID:      storj.NodeID{123},
						Address: "127.0.0.1:100",
					},
					{
						ID:      storj.NodeID{},
						Address: planet.StorageNodes[1].Addr(),
					},
				}

				for _, target := range targets {
					tag := fmt.Sprintf("%+v", target)

					timedCtx, cancel := context.WithTimeout(ctx, time.Second)
					conn, err := dialer.DialNodeURL(rpcpool.WithForceDial(timedCtx), target)
					cancel()
					assert.Error(t, err, tag)
					assert.Nil(t, conn, tag)
				}
			})

			t.Run("DialNode with valid signed target", func(t *testing.T) {
				timedCtx, cancel := context.WithTimeout(ctx, time.Second)
				conn, err := dialer.DialNodeURL(rpcpool.WithForceDial(timedCtx), planet.StorageNodes[1].NodeURL())
				cancel()

				assert.NoError(t, err)
				require.NotNil(t, conn)

				assert.NoError(t, conn.Close())
			})

			t.Run("DialNode with unsigned identity", func(t *testing.T) {
				timedCtx, cancel := context.WithTimeout(ctx, time.Second)
				conn, err := unsignedDialer.DialNodeURL(rpcpool.WithForceDial(timedCtx), planet.StorageNodes[1].NodeURL())
				cancel()

				assert.NotNil(t, conn)
				require.NoError(t, err)
				assert.NoError(t, conn.Close())
			})

			t.Run("DialAddress with unsigned identity", func(t *testing.T) {
				timedCtx, cancel := context.WithTimeout(ctx, time.Second)
				conn, err := unsignedDialer.DialAddressInsecure(rpcpool.WithForceDial(timedCtx), planet.StorageNodes[1].Addr())
				cancel()

				assert.NotNil(t, conn)
				require.NoError(t, err)
				assert.NoError(t, conn.Close())
			})

			t.Run("DialAddress with valid address", func(t *testing.T) {
				timedCtx, cancel := context.WithTimeout(ctx, time.Second)
				conn, err := dialer.DialAddressInsecure(rpcpool.WithForceDial(timedCtx), planet.StorageNodes[1].Addr())
				cancel()

				assert.NoError(t, err)
				require.NotNil(t, conn)
				assert.NoError(t, conn.Close())
			})

		}

		// test with tcp
		t.Run("TCP", func(t *testing.T) {
			test(t, ctx, planet, tcpDialer, unsignedTCPDialer)
		})
		// test with quic
		t.Run("QUIC", func(t *testing.T) {
			test(t, ctx, planet, quicDialer, unsignedQUICDialer)
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

		tcpDialer := rpc.NewDefaultDialer(tlsOptions)
		quicDialer := rpc.NewDefaultDialer(tlsOptions)
		quicDialer.Connector = quic.NewDefaultConnector(nil)

		test := func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, dialer rpc.Dialer) {
			t.Run("DialNodeURL with bad server certificate", func(t *testing.T) {
				timedCtx, cancel := context.WithTimeout(ctx, time.Second)
				conn, err := dialer.DialNodeURL(rpcpool.WithForceDial(timedCtx), planet.StorageNodes[1].NodeURL())
				cancel()

				tag := fmt.Sprintf("%+v", planet.StorageNodes[1].NodeURL())
				assert.Nil(t, conn, tag)
				require.Error(t, err, tag)
				assert.Contains(t, err.Error(), "not signed by any CA in the whitelist")
			})

			t.Run("DialAddress with bad server certificate", func(t *testing.T) {
				timedCtx, cancel := context.WithTimeout(ctx, time.Second)
				conn, err := dialer.DialNodeURL(rpcpool.WithForceDial(timedCtx), planet.StorageNodes[1].NodeURL())
				cancel()

				assert.Nil(t, conn)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "not signed by any CA in the whitelist")
			})
		}

		// test with tcp
		t.Run("TCP", func(t *testing.T) {
			test(t, ctx, planet, tcpDialer)
		})
		// test with quic
		t.Run("QUIC", func(t *testing.T) {
			test(t, ctx, planet, quicDialer)
		})
	})
}
