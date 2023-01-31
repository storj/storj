// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package server_test

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/errs2"
	"storj.io/common/identity/testidentity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	_ "storj.io/common/rpc/quic"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/server"
	"storj.io/storj/private/testplanet"
)

func TestServer(t *testing.T) {
	ctx := testcontext.New(t)
	log := zaptest.NewLogger(t)
	identity := testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion())

	host := "127.0.0.1"
	if hostlist := os.Getenv("STORJ_TEST_HOST"); hostlist != "" {
		host, _, _ = strings.Cut(hostlist, ";")
	}

	tlsOptions, err := tlsopts.NewOptions(identity, tlsopts.Config{
		PeerIDVersions: "latest",
	}, nil)
	require.NoError(t, err)

	instance, err := server.New(log.Named("server"), tlsOptions, server.Config{
		Address:          host + ":0",
		PrivateAddress:   host + ":0",
		TCPFastOpen:      true,
		TCPFastOpenQueue: 256,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, instance.Close())
	}()

	serverCtx, serverCancel := context.WithTimeout(ctx, time.Second)
	defer serverCancel()

	err = instance.Run(serverCtx)
	err = errs2.IgnoreCanceled(err)
	require.NoError(t, err)
}

func TestHybridConnector_Basic(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		dialer := planet.Uplinks[0].Dialer

		dialer.Connector = rpc.NewHybridConnector()

		conn, err := dialer.Connector.DialContext(ctx, dialer.TLSOptions.ClientTLSConfig(sat.ID()), sat.Addr())
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})
}

func TestHybridConnector_QUICOnly(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      0,
		Reconfigure:      testplanet.DisableTCP,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		identity, err := planet.NewIdentity()
		require.NoError(t, err)

		tlsOptions, err := tlsopts.NewOptions(identity, tlsopts.Config{
			PeerIDVersions: strconv.Itoa(int(storj.LatestIDVersion().Number)),
		}, nil)
		require.NoError(t, err)

		connector := rpc.NewHybridConnector()

		conn, err := connector.DialContext(ctx, tlsOptions.ClientTLSConfig(sat.ID()), sat.Addr())
		require.NoError(t, err)
		require.Equal(t, "udp", conn.LocalAddr().Network())
		require.NoError(t, conn.Close())
	})
}

func TestHybridConnector_TCPOnly(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      0,
		Reconfigure:      testplanet.DisableQUIC,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		identity, err := planet.NewIdentity()
		require.NoError(t, err)

		tlsOptions, err := tlsopts.NewOptions(identity, tlsopts.Config{
			PeerIDVersions: strconv.Itoa(int(storj.LatestIDVersion().Number)),
		}, nil)
		require.NoError(t, err)

		connector := rpc.NewHybridConnector()

		conn, err := connector.DialContext(ctx, tlsOptions.ClientTLSConfig(sat.ID()), sat.Addr())
		require.NoError(t, err)
		require.Equal(t, "tcp", conn.LocalAddr().Network())
		require.NoError(t, conn.Close())
	})
}
