// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tlsopts_test

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func TestVerifyIdentity_success(t *testing.T) {
	for i := 0; i < 50; i++ {
		ident, err := testidentity.PregeneratedIdentity(i, storj.LatestIDVersion())
		require.NoError(t, err)

		err = tlsopts.VerifyIdentity(ident.ID)(nil, identity.ToChains(ident.Chain()))
		assert.NoError(t, err)
	}
}

func TestVerifyIdentity_success_signed(t *testing.T) {
	for i := 0; i < 50; i++ {
		ident, err := testidentity.PregeneratedSignedIdentity(i, storj.LatestIDVersion())
		require.NoError(t, err)

		err = tlsopts.VerifyIdentity(ident.ID)(nil, identity.ToChains(ident.Chain()))
		assert.NoError(t, err)
	}
}

func TestVerifyIdentity_error(t *testing.T) {
	ident, err := testidentity.PregeneratedIdentity(0, storj.LatestIDVersion())
	require.NoError(t, err)

	identTheftVictim, err := testidentity.PregeneratedIdentity(1, storj.LatestIDVersion())
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
	// TODO: rename and/or move test
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	revokedLeafKeys, revokedLeafChain, _, err := testpeertls.NewRevokedLeafChain()
	assert.NoError(t, err)

	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
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
					chain, revocationExt, err := testpeertls.RevokeLeaf(revokedLeafKeys[peertls.CAIndex], revokedLeafChain)
					assert.NoError(t, err)

					err = rev.Unmarshal(revocationExt.Value)
					assert.NoError(t, err)

					return chain
				}(),
			},
		}

		for _, testcase := range testcases {
			t.Run(testcase.name, func(t *testing.T) {
				opts := &extensions.Options{
					RevDB: revDB,
					PeerIDVersions: "latest",
				}

				handlerFuncMap := extensions.AllHandlers.WithOptions(opts)
				extensionsMap := tlsopts.NewExtensionsMap(testcase.certChain...)
				err := extensionsMap.HandleExtensions(handlerFuncMap, identity.ToChains(testcase.certChain))
				assert.NoError(t, err)
			})
		}
	})
}

func TestExtensionMap_HandleExtensions_error(t *testing.T) {
	// TODO: rename and/or move test
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
		keys, chain, oldRevocation, err := testpeertls.NewRevokedLeafChain()
		assert.NoError(t, err)

		// NB: node ID is the same, timestamp must change
		// (see: identity.RevocationDB#Put)
		time.Sleep(time.Second)
		_, newRevocation, err := testpeertls.RevokeLeaf(keys[peertls.CAIndex], chain)
		require.NoError(t, err)

		assert.NotEqual(t, oldRevocation, newRevocation)

		err = revDB.Put(chain, newRevocation)
		assert.NoError(t, err)

		extOpts := &extensions.Options{RevDB: revDB}
		handlerMap := extensions.HandlerFactories{
			extensions.RevocationUpdateHandler,
		}.WithOptions(extOpts)
		extensionsMap := tlsopts.NewExtensionsMap(chain[peertls.LeafIndex])

		assert.Equal(t, oldRevocation, extensionsMap[extensions.RevocationExtID.String()])

		err = extensionsMap.HandleExtensions(handlerMap, identity.ToChains(chain))
		assert.Errorf(t, err, extensions.ErrRevocationTimestamp.Error())
	})
}
