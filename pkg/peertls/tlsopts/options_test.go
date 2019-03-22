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
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

func TestNewOptions(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fi, err := testplanet.PregeneratedIdentity(0)
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
			0, 0,
		}, {
			"revocation processing",
			tlsopts.Config{
				RevocationDBURL: "bolt://" + ctx.File("revocation1.db"),
				Extensions: peertls.TLSExtConfig{
					Revocation: true,
				},
			},
			2, 2,
		}, {
			"ca whitelist verification",
			tlsopts.Config{
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
			},
			1, 0,
		}, {
			"ca whitelist verification and whitelist signed leaf verification",
			tlsopts.Config{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
				Extensions: peertls.TLSExtConfig{
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
				Extensions: peertls.TLSExtConfig{
					Revocation: true,
				},
			},
			3, 2,
		}, {
			"revocation processing, whitelist, and signed leaf verification",
			tlsopts.Config{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
				RevocationDBURL:     "bolt://" + ctx.File("revocation3.db"),
				Extensions: peertls.TLSExtConfig{
					Revocation:          true,
					WhitelistSignedLeaf: true,
				},
			},
			3, 2,
		},
	}

	for _, c := range cases {
		t.Log(c.testID)
		opts, err := tlsopts.NewOptions(fi, c.config)
		assert.NoError(t, err)
		assert.True(t, reflect.DeepEqual(fi, opts.Ident))
		assert.Equal(t, c.config, opts.Config)
		assert.Len(t, opts.VerificationFuncs.Client(), c.clientVerificationFuncsLen)
		assert.Len(t, opts.VerificationFuncs.Server(), c.serverVerificationFuncsLen)
	}
}

type identFunc func(int) (*identity.FullIdentity, error)

func TestOptions_ServerOption_Peer_CA_Whitelist(t *testing.T) {
	ctx := testcontext.New(t)

	planet, err := testplanet.New(t, 0, 2, 0)
	require.NoError(t, err)

	planet.Start(ctx)
	defer ctx.Check(planet.Shutdown)

	target := planet.StorageNodes[1].Local()

	testCases := []struct {
		name   string
		identF identFunc
	}{
		{"unsigned client identity", testplanet.PregeneratedIdentity},
		{"signed client identity", testplanet.PregeneratedSignedIdentity},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ident, err := testCase.identF(0)
			require.NoError(t, err)

			opts, err := tlsopts.NewOptions(ident, tlsopts.Config{})
			require.NoError(t, err)

			dialOption, err := opts.DialOption(target.Id)
			require.NoError(t, err)

			transportClient := transport.NewClient(opts)

			conn, err := transportClient.DialNode(ctx, &target, dialOption)
			assert.NotNil(t, conn)
			assert.NoError(t, err)
		})
	}
}

func TestOptions_DialOption_error_on_empty_ID(t *testing.T) {
	ident, err := testplanet.PregeneratedIdentity(0)
	require.NoError(t, err)

	opts, err := tlsopts.NewOptions(ident, tlsopts.Config{})
	require.NoError(t, err)

	dialOption, err := opts.DialOption(storj.NodeID{})
	assert.Nil(t, dialOption)
	assert.Error(t, err)
}

func TestOptions_DialUnverifiedIDOption(t *testing.T) {
	ident, err := testplanet.PregeneratedIdentity(0)
	require.NoError(t, err)

	opts, err := tlsopts.NewOptions(ident, tlsopts.Config{})
	require.NoError(t, err)

	dialOption := opts.DialUnverifiedIDOption()
	assert.NotNil(t, dialOption)
}
