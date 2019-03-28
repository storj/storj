// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity_test

import (
	"bytes"
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
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func TestRevocationDB_Get(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		require.NoError(t, err)

		ext, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
		require.NoError(t, err)

		var rev *extensions.Revocation

		{
			t.Log("missing key")
			rev, err = revDB.Get(chain)
			assert.NoError(t, err)
			assert.Nil(t, rev)

			nodeID, err := identity.NodeIDFromCert(chain[peertls.CAIndex])
			require.NoError(t, err)

			err = db.Put(nodeID.Bytes(), ext.Value)
			require.NoError(t, err)
		}

		{
			t.Log("existing key")
			rev, err = revDB.Get(chain)
			assert.NoError(t, err)

			revBytes, err := rev.Marshal()
			assert.NoError(t, err)
			assert.True(t, bytes.Equal(ext.Value, revBytes))
		}
	})
}

func TestRevocationDB_Put_success(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		require.NoError(t, err)

		firstRevocation, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
		require.NoError(t, err)

		// NB: revocation timestamps need to be different between revocations for the same
		// identity to be valid.
		time.Sleep(time.Second)
		newerRevocation, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
		assert.NoError(t, err)

		testcases := []struct {
			name string
			ext  pkix.Extension
		}{
			{
				"new key",
				firstRevocation,
			},
			{
				"existing key - newer timestamp",
				newerRevocation,
			},
			// TODO(bryanchriswhite): test empty/garbage cert/timestamp/sig
		}

		for _, testcase := range testcases {
			t.Log(testcase.name)
			require.NotNil(t, testcase.ext)

			err = revDB.Put(chain, testcase.ext)
			require.NoError(t, err)

			nodeID, err := identity.NodeIDFromCert(chain[peertls.CAIndex])
			require.NoError(t, err)

			revBytes, err := db.Get(nodeID.Bytes())
			require.NoError(t, err)

			assert.Equal(t, testcase.ext.Value, []byte(revBytes))
		}
	})
}

func TestRevocationDB_Put_error(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testidentity.RevocationDBsTest(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		require.NoError(t, err)

		olderRevocation, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
		assert.NoError(t, err)

		time.Sleep(time.Second)
		newerRevocation, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
		require.NoError(t, err)

		err = revDB.Put(chain, newerRevocation)
		require.NoError(t, err)

		testcases := []struct {
			name string
			ext  pkix.Extension
			err  error
		}{
			{
				"existing key - older timestamp",
				olderRevocation,
				extensions.ErrRevocationTimestamp,
			},
			// TODO(bryanchriswhite): test empty/garbage cert/timestamp/sig
		}

		for _, testcase := range testcases {
			t.Log(testcase.name)
			require.NotNil(t, testcase.ext)

			err = revDB.Put(chain, testcase.ext)
			assert.True(t, extensions.Error.Has(err))
			assert.Equal(t, testcase.err, err)
		}
	})
}
