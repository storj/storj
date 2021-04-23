// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package revocation_test

import (
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity"
	"storj.io/common/peertls"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/testpeertls"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testrevocation"
	"storj.io/storj/storage"
)

func TestRevocationCheckHandler(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testrevocation.RunDBs(t, func(t *testing.T, revDB extensions.RevocationDB, _ storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		assert.NoError(t, err)

		opts := &extensions.Options{RevocationDB: revDB}
		revocationChecker := extensions.RevocationCheckHandler.NewHandlerFunc(opts)

		revokingChain, leafRevocationExt, err := testpeertls.RevokeLeaf(keys[peertls.CAIndex], chain)
		require.NoError(t, err)

		assert.Equal(t, chain[peertls.CAIndex].Raw, revokingChain[peertls.CAIndex].Raw)

		{
			t.Log("revoked leaf success (original chain)")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(chain))
			assert.NoError(t, err)
		}

		{
			t.Log("revoked leaf success (revoking chain)")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(revokingChain))
			assert.NoError(t, err)
		}

		// NB: add leaf revocation to revocation DB
		t.Log("revocation DB put leaf revocation")
		err = revDB.Put(ctx, revokingChain, leafRevocationExt)
		require.NoError(t, err)

		{
			t.Log("revoked leaf success (revoking chain)")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(revokingChain))
			assert.NoError(t, err)
		}

		{
			t.Log("revoked leaf error (original chain)")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(chain))
			assert.Error(t, err)
		}
	})

	testrevocation.RunDBs(t, func(t *testing.T, revDB extensions.RevocationDB, _ storage.KeyValueStore) {
		t.Log("new revocation DB")
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		assert.NoError(t, err)

		opts := &extensions.Options{RevocationDB: revDB}
		revocationChecker := extensions.RevocationCheckHandler.NewHandlerFunc(opts)
		revokingChain, caRevocationExt, err := testpeertls.RevokeCA(keys[peertls.CAIndex], chain)
		require.NoError(t, err)

		assert.NotEqual(t, chain[peertls.CAIndex].Raw, revokingChain[peertls.CAIndex].Raw)

		chainID, err := identity.NodeIDFromCert(chain[peertls.CAIndex])
		require.NoError(t, err)

		revokingChainID, err := identity.NodeIDFromCert(revokingChain[peertls.CAIndex])
		require.NoError(t, err)

		assert.Equal(t, chainID, revokingChainID)

		{
			t.Log("revoked CA error (original chain)")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(chain))
			assert.NoError(t, err)
		}

		{
			t.Log("revoked CA success (revokingChain)")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(revokingChain))
			assert.NoError(t, err)
		}

		// NB: add CA revocation to revocation DB
		t.Log("revocation DB put CA revocation")
		err = revDB.Put(ctx, revokingChain, caRevocationExt)
		require.NoError(t, err)

		{
			t.Log("revoked CA error (revoking CA chain)")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(revokingChain))
			assert.Error(t, err)
		}

		{
			t.Log("revoked CA error (original chain)")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(chain))
			assert.Error(t, err)
		}
	})
}

func TestRevocationUpdateHandler(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testrevocation.RunDBs(t, func(t *testing.T, revDB extensions.RevocationDB, _ storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		assert.NoError(t, err)

		olderRevokedChain, olderRevocation, err := testpeertls.RevokeLeaf(keys[peertls.CAIndex], chain)
		require.NoError(t, err)

		time.Sleep(time.Second)
		revokedLeafChain, newerRevocation, err := testpeertls.RevokeLeaf(keys[peertls.CAIndex], chain)
		require.NoError(t, err)

		time.Sleep(time.Second)
		newestRevokedChain, newestRevocation, err := testpeertls.RevokeLeaf(keys[peertls.CAIndex], revokedLeafChain)
		require.NoError(t, err)

		opts := &extensions.Options{RevocationDB: revDB}
		revocationChecker := extensions.RevocationUpdateHandler.NewHandlerFunc(opts)

		{
			t.Log("first revocation")
			err := revocationChecker(newerRevocation, identity.ToChains(revokedLeafChain))
			assert.NoError(t, err)
		}
		{
			t.Log("older revocation error")
			err = revocationChecker(olderRevocation, identity.ToChains(olderRevokedChain))
			assert.Error(t, err)
		}
		{
			t.Log("newer revocation")
			err = revocationChecker(newestRevocation, identity.ToChains(newestRevokedChain))
			assert.NoError(t, err)
		}
	})
}

func TestWithOptions_NilRevocationDB(t *testing.T) {
	_, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
	require.NoError(t, err)

	opts := &extensions.Options{RevocationDB: nil}
	handlerFuncMap := extensions.DefaultHandlers.WithOptions(opts)

	extMap := tlsopts.NewExtensionsMap(chain[peertls.LeafIndex])
	err = extMap.HandleExtensions(handlerFuncMap, identity.ToChains(chain))
	require.NoError(t, err)
}
