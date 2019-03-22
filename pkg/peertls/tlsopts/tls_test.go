// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tlsopts_test

import (
	"crypto/x509"
	"testing"
	"time"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/pkg/peertls/extensions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
)

func TestVerifyIdentity_success(t *testing.T) {
	for i := 0; i < 50; i++ {
		ident, err := testplanet.PregeneratedIdentity(i)
		require.NoError(t, err)

		err = tlsopts.VerifyIdentity(ident.ID)(nil, identity.ToChains(ident.Chain()))
		assert.NoError(t, err)
	}
}

func TestVerifyIdentity_success_signed(t *testing.T) {
	for i := 0; i < 50; i++ {
		ident, err := testplanet.PregeneratedSignedIdentity(i)
		require.NoError(t, err)

		err = tlsopts.VerifyIdentity(ident.ID)(nil, identity.ToChains(ident.Chain()))
		assert.NoError(t, err)
	}
}

func TestVerifyIdentity_error(t *testing.T) {
	ident, err := testplanet.PregeneratedIdentity(0)
	require.NoError(t, err)

	identTheftVictim, err := testplanet.PregeneratedIdentity(1)
	require.NoError(t, err)

	cases := []struct {
		test   string
		nodeID storj.NodeID
	}{
		{"empty node ID", storj.NodeID{}},
		{"garbage node ID", storj.NodeID{0, 1, 2, 3}},
		{"wrong node ID", identTheftVictim.ID},
	}

	for _, c := range cases {
		t.Run(c.test, func(t *testing.T) {
			err := tlsopts.VerifyIdentity(c.nodeID)(nil, identity.ToChains(ident.Chain()))
			assert.Error(t, err)
		})
	}
}

func TestExtensionMap_HandleExtensions(t *testing.T) {
	// TODO: separate this into multiple tests!
	// TODO: this is not a great test
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	revokedLeafKeys, revokedLeafChain, _, err := testpeertls.NewRevokedLeafChain()
	assert.NoError(t, err)

	revDB, err := identity.NewRevocationDBBolt(ctx.File("revocations.db"))
	assert.NoError(t, err)
	defer ctx.Check(revDB.Close)

	testcases := []struct {
		name      string
		config    extensions.Config
		certChain []*x509.Certificate
	}{
		{
			"certificate revocation - single revocation ",
			extensions.Config{Revocation: true},
			revokedLeafChain,
		},
		{
			"certificate revocation - serial revocations",
			extensions.Config{Revocation: true},
			func() []*x509.Certificate {
				rev := new(extensions.Revocation)
				time.Sleep(1 * time.Second)
				chain, revocationExt, err := testpeertls.RevokeLeaf(revokedLeafKeys, revokedLeafChain)
				assert.NoError(t, err)

				err = rev.Unmarshal(revocationExt.Value)
				assert.NoError(t, err)

				return chain
			}(),
		},
		{
			"certificate revocation",
			extensions.Config{Revocation: true, WhitelistSignedLeaf: true},
			func() []*x509.Certificate {
				_, chain, _, err := testpeertls.NewRevokedLeafChain()
				assert.NoError(t, err)

				return chain
			}(),
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			opts := &extensions.Options{
				RevDB: revDB,
			}

			handlerFuncMap := extensions.AllHandlers.WithOptions(opts)
			extensionsMap := tlsopts.NewExtensionsMap(testcase.certChain...)
			err := extensionsMap.HandleExtensions(handlerFuncMap, identity.ToChains(testcase.certChain))
			assert.NoError(t, err)
		})
	}
}

