// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tlsopts_test

import (
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
)

func TestNewOptions(t *testing.T) {
	// TODO: this is not a great test...
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fi, err := testidentity.PregeneratedIdentity(0, storj.LatestIDVersion())
	require.NoError(t, err)

	whitelistPath := ctx.File("whitelist.pem")

	chainData, err := peertls.ChainBytes(fi.CA)
	assert.NoError(t, err)

	err = ioutil.WriteFile(whitelistPath, chainData, 0644)
	assert.NoError(t, err)

	cases := []struct {
		testID                     string
		config                     tlsopts.Config
		clientVerificationFuncsLen int
		serverVerificationFuncsLen int
	}{
		{
			"default",
			tlsopts.Config{},
			1, 1,
		}, {
			"revocation processing",
			tlsopts.Config{
				RevocationDBURL: "bolt://" + ctx.File("revocation1.db"),
				Extensions: extensions.Config{
					Revocation: true,
				},
			},
			1, 1,
		}, {
			"ca whitelist verification",
			tlsopts.Config{
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
			},
			2, 1,
		}, {
			"ca whitelist verification and whitelist signed leaf verification",
			tlsopts.Config{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
				Extensions: extensions.Config{
					WhitelistSignedLeaf: true,
				},
			},
			2, 1,
		}, {
			"revocation processing and whitelist verification",
			tlsopts.Config{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
				RevocationDBURL:     "bolt://" + ctx.File("revocation2.db"),
				Extensions: extensions.Config{
					Revocation: true,
				},
			},
			2, 1,
		}, {
			"revocation processing, whitelist, and signed leaf verification",
			tlsopts.Config{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
				RevocationDBURL:     "bolt://" + ctx.File("revocation3.db"),
				Extensions: extensions.Config{
					Revocation:          true,
					WhitelistSignedLeaf: true,
				},
			},
			2, 1,
		},
	}

	for _, c := range cases {
		t.Log(c.testID)

		revocationDB, err := revocation.NewDBFromCfg(c.config)
		require.NoError(t, err)

		tlsOptions, err := tlsopts.NewOptions(fi, c.config, revocationDB)
		assert.NoError(t, err)
		assert.True(t, reflect.DeepEqual(fi, tlsOptions.Ident))
		assert.Equal(t, c.config, tlsOptions.Config)
		assert.Len(t, tlsOptions.VerificationFuncs.Client(), c.clientVerificationFuncsLen)
		assert.Len(t, tlsOptions.VerificationFuncs.Server(), c.serverVerificationFuncsLen)

		require.NoError(t, revocationDB.Close())
	}
}

func TestOptions_ServerOption_Peer_CA_Whitelist(t *testing.T) {
	ctx := testcontext.New(t)

	planet, err := testplanet.New(t, 0, 2, 0)
	require.NoError(t, err)

	planet.Start(ctx)
	defer ctx.Check(planet.Shutdown)

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
}

func TestOptions_DialOption_error_on_empty_ID(t *testing.T) {
	testidentity.CompleteIdentityVersionsTest(t, func(t *testing.T, version storj.IDVersion, ident *identity.FullIdentity) {
		tlsOptions, err := tlsopts.NewOptions(ident, tlsopts.Config{
			PeerIDVersions: "*",
		}, nil)
		require.NoError(t, err)

		dialOption, err := tlsOptions.DialOption(storj.NodeID{})
		assert.Nil(t, dialOption)
		assert.Error(t, err)
	})
}

func TestOptions_DialUnverifiedIDOption(t *testing.T) {
	testidentity.CompleteIdentityVersionsTest(t, func(t *testing.T, version storj.IDVersion, ident *identity.FullIdentity) {
		tlsOptions, err := tlsopts.NewOptions(ident, tlsopts.Config{
			PeerIDVersions: "*",
		}, nil)
		require.NoError(t, err)

		dialOption := tlsOptions.DialUnverifiedIDOption()
		assert.NotNil(t, dialOption)
	})
}
