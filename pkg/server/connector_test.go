// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package server_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/pkg/server"
	"storj.io/storj/private/testplanet"
)

func TestHybridConnector_Basic(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		dialer := planet.Uplinks[0].Dialer

		dialer.Connector = server.NewDefaultHybridConnector(nil, nil)

		_, err := dialer.Connector.DialContext(ctx, dialer.TLSOptions.ClientTLSConfig(sat.ID()), sat.Addr())
		require.NoError(t, err)
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

		connector := server.NewDefaultHybridConnector(nil, nil)

		conn, err := connector.DialContext(ctx, tlsOptions.ClientTLSConfig(sat.ID()), sat.Addr())
		require.NoError(t, err)
		require.Equal(t, "udp", conn.LocalAddr().Network())
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

		connector := server.NewDefaultHybridConnector(nil, nil)

		conn, err := connector.DialContext(ctx, tlsOptions.ClientTLSConfig(sat.ID()), sat.Addr())
		require.NoError(t, err)
		require.Equal(t, "tcp", conn.LocalAddr().Network())
	})
}
