// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package extensions_test

import (
	"crypto/x509/pkix"
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
	"storj.io/storj/storage"
)

func TestRevocationCheckHandler(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, _ storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2)
		assert.NoError(t, err)

		opts := &extensions.Options{RevDB: revDB}
		revocationChecker := extensions.RevocationCheckHandler.NewHandlerFunc(opts)

		{
			t.Log("no revocations")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(chain))
			assert.NoError(t, err)
		}

		revokedLeafChain, leafRevocationExt, err := testpeertls.RevokeLeaf(keys, chain)
		require.NoError(t, err)

		assert.Equal(t, chain[peertls.CAIndex].Raw, revokedLeafChain[peertls.CAIndex].Raw)

		{
			t.Log("revoked leaf success")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(revokedLeafChain))
			assert.NoError(t, err)
		}

		// NB: add leaf revocation to revocation DB
		err = revDB.Put(revokedLeafChain, leafRevocationExt)
		require.NoError(t, err)

		{
			t.Log("revoked leaf error")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(chain))
			assert.Error(t, err)
		}

		// NB: timestamp must be different because the NodeID is the same
		time.Sleep(time.Second)

		revokedCAChain, caRevocationExt, err := testpeertls.RevokeCA(keys, chain)
		require.NoError(t, err)

		{
			t.Log("revoked CA success")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(revokedCAChain))
			assert.NoError(t, err)
		}

		// NB: add CA revocation to revocation DB
		err = revDB.Put(revokedCAChain, caRevocationExt)
		require.NoError(t, err)

		{
			t.Log("revoked CA error")
			err := revocationChecker(pkix.Extension{}, identity.ToChains(revokedCAChain))
			assert.Error(t, err)
		}
	})
}

func TestRevocationUpdateHandler(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, _ storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2)
		assert.NoError(t, err)

		olderRevokedChain, olderRevocation, err := testpeertls.RevokeLeaf(keys, chain)
		require.NoError(t, err)

		time.Sleep(time.Second)
		revokedLeafChain, newerRevocation, err := testpeertls.RevokeLeaf(keys, chain)
		require.NoError(t, err)

		time.Sleep(time.Second)
		newestRevokedChain, newestRevocation, err := testpeertls.RevokeLeaf(keys, revokedLeafChain)
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
