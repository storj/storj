// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestCertDB(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		certdb := db.CertDB()

		node0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		node1 := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion())

		certid0, err := certdb.Include(ctx, node0.PeerIdentity())
		require.NoError(t, err)

		certid1, err := certdb.Include(ctx, node1.PeerIdentity())
		require.NoError(t, err)

		certid0duplicate, err := certdb.Include(ctx, node0.PeerIdentity())
		require.NoError(t, err)

		require.Equal(t, certid0, certid0duplicate, "insert duplicate")
		require.NotEqual(t, certid0, certid1, "insert non-duplicate")

		identity, err := certdb.LookupByCertID(ctx, certid0)
		require.NoError(t, err, "lookup by id")

		require.Equal(t, node0.PeerIdentity(), identity)
	})
}
