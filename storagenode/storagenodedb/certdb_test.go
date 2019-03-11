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

	id0, err := certdb.Include(ctx, node0.ID, node0.PeerIdentity().Leaf.Raw)
	require.NoError(t, err)

	id0other, err := certdb.Include(ctx, node0.ID, node0.PeerIdentity().CA.Raw)
	require.NoError(t, err)

	id0dup, err := certdb.Include(ctx, node0.ID, node0.PeerIdentity().Leaf.Raw)
	require.NoError(t, err)

	require.Equal(t, id0, id0dup)
	require.NotEqual(t, id0, id0other)

	cert, err := certdb.LookupByCertID(ctx, id0)
	require.NoError(t, err)

	require.Equal(t, node0.PeerIdentity().Leaf.Raw, cert)
}
