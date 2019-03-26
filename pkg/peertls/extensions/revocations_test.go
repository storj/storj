// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package extensions_test

import (
	"crypto/x509/pkix"
	"storj.io/storj/pkg/storj"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/storage"
)

func TestRevocationCheckHandler(t *testing.T) {
	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, _ storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		assert.NoError(t, err)

		opts := &extensions.Options{RevDB: revDB}
		revocationChecker := extensions.RevocationCheckHandler.NewHandlerFunc(opts)

		{
			t.Log("no revocations")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(chain))
			assert.NoError(t, err)
		}

		revokingChain, leafRevocationExt, err := testpeertls.RevokeLeaf(keys[0], chain)
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
		err = revDB.Put(revokingChain, leafRevocationExt)
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

	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, _ storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		assert.NoError(t, err)

		opts := &extensions.Options{RevDB: revDB}
		revocationChecker := extensions.RevocationCheckHandler.NewHandlerFunc(opts)
		revokingChain, caRevocationExt, err := testpeertls.RevokeCA(keys[0], chain)
		require.NoError(t, err)

		assert.NotEqual(t, chain[peertls.CAIndex].Raw, revokingChain[peertls.CAIndex].Raw)

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
		err = revDB.Put(revokingChain, caRevocationExt)
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
	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, _ storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		assert.NoError(t, err)

		olderRevokedChain, olderRevocation, err := testpeertls.RevokeLeaf(keys[0], chain)
		require.NoError(t, err)

		time.Sleep(time.Second)
		revokedLeafChain, newerRevocation, err := testpeertls.RevokeLeaf(keys[0], chain)
		require.NoError(t, err)

		time.Sleep(time.Second)
		newestRevokedChain, newestRevocation, err := testpeertls.RevokeLeaf(keys[0], revokedLeafChain)
		require.NoError(t, err)

		opts := &extensions.Options{RevDB: revDB}
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
