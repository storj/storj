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
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func TestVerifyIdentity_success(t *testing.T) {
	for i := 0; i < 50; i++ {
		ident, err := testplanet.PregeneratedIdentity(i, storj.LatestIDVersion())
		require.NoError(t, err)

		err = tlsopts.VerifyIdentity(ident.ID)(nil, identity.ToChains(ident.Chain()))
		assert.NoError(t, err)
	}
}

func TestVerifyIdentity_success_signed(t *testing.T) {
	for i := 0; i < 50; i++ {
		ident, err := testplanet.PregeneratedSignedIdentity(i, storj.LatestIDVersion())
		require.NoError(t, err)

		err = tlsopts.VerifyIdentity(ident.ID)(nil, identity.ToChains(ident.Chain()))
		assert.NoError(t, err)
	}
}

func TestVerifyIdentity_error(t *testing.T) {
	ident, err := testplanet.PregeneratedIdentity(0, storj.LatestIDVersion())
	require.NoError(t, err)

	identTheftVictim, err := testplanet.PregeneratedIdentity(1, storj.LatestIDVersion())
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
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	keys, originalChain, err := testpeertls.NewCertChain(2)
	assert.NoError(t, err)

	rev := new(extensions.Revocation)

	// TODO: `keys[peertls.CAIndex]`
	oldRevokedLeafChain, revocationExt, err := testpeertls.RevokeLeaf(keys[0], originalChain)
	require.NoError(t, err)
	err = rev.Unmarshal(revocationExt.Value)
	require.NoError(t, err)
	err = rev.Verify(oldRevokedLeafChain[peertls.CAIndex])
	require.NoError(t, err)

	// NB: node ID is the same, timestamp must change
	// (see: identity.RevocationDB#Put)
	time.Sleep(1 * time.Second)
	// TODO: `keys[peertls.CAIndex]`
	newRevokedLeafChain, revocationExt, err := testpeertls.RevokeLeaf(keys[0], oldRevokedLeafChain)
	require.NoError(t, err)
	err = rev.Unmarshal(revocationExt.Value)
	require.NoError(t, err)
	err = rev.Verify(newRevokedLeafChain[peertls.CAIndex])
	require.NoError(t, err)

	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
		opts := &extensions.Options{
			RevDB: revDB,
		}

		testcases := []struct {
			name  string
			chain []*x509.Certificate
		}{
			{"no extensions", originalChain},
			{"leaf revocation", oldRevokedLeafChain},
			{"double leaf revocation", newRevokedLeafChain},
			// TODO: more and more diverse extensions in cases
		}

		{
			handlerFuncMap := extensions.AllHandlers.WithOptions(opts)
			for _, testcase := range testcases {
				t.Log(testcase.name)
				extensionsMap := tlsopts.NewExtensionsMap(testcase.chain...)
				err := extensionsMap.HandleExtensions(handlerFuncMap, identity.ToChains(testcase.chain))
				assert.NoError(t, err)
			}
		}
	})
}

func TestExtensionMap_HandleExtensions_error(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
		keys, chain, oldRevocation, err := testpeertls.NewRevokedLeafChain()
		assert.NoError(t, err)

		// NB: node ID is the same, timestamp must change
		// (see: identity.RevocationDB#Put)
		time.Sleep(time.Second)
		_, newRevocation, err := testpeertls.RevokeLeaf(keys[0], chain)
		require.NoError(t, err)

		assert.NotEqual(t, oldRevocation, newRevocation)

		err = revDB.Put(chain, newRevocation)
		assert.NoError(t, err)

		opts := &extensions.Options{RevDB: revDB}
		handlerFuncMap := extensions.HandlerFactories{
			extensions.RevocationUpdateHandler,
		}.WithOptions(opts)
		extensionsMap := tlsopts.NewExtensionsMap(chain[peertls.LeafIndex])

		assert.Equal(t, oldRevocation, extensionsMap[extensions.RevocationExtID.String()])

		err = extensionsMap.HandleExtensions(handlerFuncMap, identity.ToChains(chain))
		assert.Errorf(t, err, extensions.ErrRevocationTimestamp.Error())
	})
}
