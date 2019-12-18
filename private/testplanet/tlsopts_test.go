// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testplanet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testidentity"
	"storj.io/storj/private/testplanet"
)

func TestOptions_ServerOption_Peer_CA_Whitelist(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 0, StorageNodeCount: 2, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		target := planet.StorageNodes[1].Local()
		testidentity.CompleteIdentityVersionsTest(t, func(t *testing.T, version storj.IDVersion, ident *identity.FullIdentity) {
			tlsOptions, err := tlsopts.NewOptions(ident, tlsopts.Config{
				PeerIDVersions: "*",
			}, nil)
			require.NoError(t, err)

			dialer := rpc.NewDefaultDialer(tlsOptions)

			conn, err := dialer.DialNode(ctx, &target.Node)
			assert.NotNil(t, conn)
			assert.NoError(t, err)

			assert.NoError(t, conn.Close())
		})
	})
}
