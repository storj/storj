// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/storagenode/storagenodedb"
)

func TestCertDB(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	db, err := storagenodedb.NewInfoInMemory()
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	require.NoError(t, db.CreateTables(log))

	certdb := db.CertDB()

	node0 := testplanet.MustPregeneratedSignedIdentity(0)
	node1 := testplanet.MustPregeneratedSignedIdentity(1)

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
}
