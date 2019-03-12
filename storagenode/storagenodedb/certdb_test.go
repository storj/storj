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

	node0 := testplanet.MustPregeneratedIdentity(0)

	idLeaf, err := certdb.Include(ctx, node0.ID, node0.PeerIdentity().Leaf.Raw)
	require.NoError(t, err)

	idCA, err := certdb.Include(ctx, node0.ID, node0.PeerIdentity().CA.Raw)
	require.NoError(t, err)

	idLeafDuplicate, err := certdb.Include(ctx, node0.ID, node0.PeerIdentity().Leaf.Raw)
	require.NoError(t, err)

	require.Equal(t, idLeaf, idLeafDuplicate, "insert duplicate Leaf")
	require.NotEqual(t, idLeaf, idCA, "insert non-duplicate CA")

	cert, err := certdb.LookupByCertID(ctx, idLeaf)
	require.NoError(t, err, "lookup by id")

	require.Equal(t, node0.PeerIdentity().Leaf.Raw, cert)
}
