// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package revocation_test

import (
	"bytes"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity"
	"storj.io/common/peertls"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/testpeertls"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testrevocation"
	"storj.io/storj/storage"
)

func TestRevocationDB_Get(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testrevocation.RunDBs(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		require.NoError(t, err)

		ext, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
		require.NoError(t, err)

		var rev *extensions.Revocation

		{
			t.Log("missing key")
			rev, err = revDB.Get(ctx, chain)
			require.NoError(t, err)
			assert.Nil(t, rev)

			nodeID, err := identity.NodeIDFromCert(chain[peertls.CAIndex])
			require.NoError(t, err)

			err = db.Put(ctx, nodeID.Bytes(), ext.Value)
			require.NoError(t, err)
		}

		{
			t.Log("existing key")
			rev, err = revDB.Get(ctx, chain)
			require.NoError(t, err)

			revBytes, err := rev.Marshal()
			require.NoError(t, err)
			assert.True(t, bytes.Equal(ext.Value, revBytes))
		}
	})
}

func TestRevocationDB_Put_success(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testrevocation.RunDBs(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		require.NoError(t, err)

		firstRevocation, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
		require.NoError(t, err)

		// NB: revocation timestamps need to be different between revocations for the same
		// identity to be valid.
		time.Sleep(time.Second)
		newerRevocation, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
		require.NoError(t, err)

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

			err = revDB.Put(ctx, chain, testcase.ext)
			require.NoError(t, err)

			nodeID, err := identity.NodeIDFromCert(chain[peertls.CAIndex])
			require.NoError(t, err)

			revBytes, err := db.Get(ctx, nodeID.Bytes())
			require.NoError(t, err)

			assert.Equal(t, testcase.ext.Value, []byte(revBytes))
		}
	})
}

func TestRevocationDB_Put_error(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testrevocation.RunDBs(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		require.NoError(t, err)

		olderRevocation, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
		require.NoError(t, err)

		time.Sleep(time.Second)
		newerRevocation, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
		require.NoError(t, err)

		err = revDB.Put(ctx, chain, newerRevocation)
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

			err = revDB.Put(ctx, chain, testcase.ext)
			assert.True(t, extensions.Error.Has(err))
			assert.Equal(t, testcase.err, err)
		}
	})
}

func TestRevocationDB_List(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testrevocation.RunDBs(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
		keys, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		require.NoError(t, err)
		keys2, chain2, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		require.NoError(t, err)

		// test list no revocations, should not error
		revs, err := revDB.List(ctx)
		require.NoError(t, err)
		assert.Nil(t, revs)

		// list 1,2 revocations
		firstRevocation, err := extensions.NewRevocationExt(keys[peertls.CAIndex], chain[peertls.LeafIndex])
		require.NoError(t, err)

		err = revDB.Put(ctx, chain, firstRevocation)
		require.NoError(t, err)
		revs, err = revDB.List(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, len(revs))
		revBytes, err := revs[0].Marshal()
		require.NoError(t, err)
		assert.True(t, bytes.Equal(firstRevocation.Value, revBytes))

		secondRevocation, err := extensions.NewRevocationExt(keys2[peertls.CAIndex], chain2[peertls.LeafIndex])
		require.NoError(t, err)
		err = revDB.Put(ctx, chain2, secondRevocation)
		require.NoError(t, err)
		revs, err = revDB.List(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, len(revs))

		expected := [][]byte{firstRevocation.Value, secondRevocation.Value}
		for _, rev := range revs {
			revBytes, err := rev.Marshal()
			require.NoError(t, err)
			assert.Contains(t, expected, revBytes)
		}
	})
}
