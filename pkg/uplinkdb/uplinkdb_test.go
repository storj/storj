// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplinkdb_test

import (
	"context"
	"crypto/ecdsa"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/uplinkdb"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUplinkDB(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		testDatabase(ctx, t, db.UplinkDB())
	})
}

func testDatabase(ctx context.Context, t *testing.T, upldb uplinkdb.DB) {
	//testing variables
	upID, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)

	{ // New entry
		err := upldb.SavePublicKey(ctx, upID.ID, upID.Leaf.PublicKey.(*ecdsa.PublicKey))
		assert.NoError(t, err)
	}

	{ // New entry
		err := upldb.SavePublicKey(ctx, upID.ID, upID.Leaf.PublicKey)
		assert.NoError(t, err)
	}

	{ // Get the corresponding Public key for the serialnum
		agreement, err := upldb.GetPublicKey(ctx, upID.ID)
		assert.NoError(t, err)
		assert.NotNil(t, agreement)
	}
}
