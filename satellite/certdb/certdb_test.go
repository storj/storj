// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package certdb_test

import (
	"context"
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/certdb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestCertDB(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		testDatabase(ctx, t, db.CertDB())
	})
}

func testDatabase(ctx context.Context, t *testing.T, upldb certdb.DB) {
	//testing variables
	upID, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	upIDpubbytes, err := x509.MarshalPKIXPublicKey(upID.Leaf.PublicKey)
	require.NoError(t, err)

	{ // New entry
		err := upldb.SavePublicKey(ctx, upID.ID, upID.Leaf.PublicKey)
		assert.NoError(t, err)
	}

	{ // New entry
		err := upldb.SavePublicKey(ctx, upID.ID, upID.Leaf.PublicKey)
		assert.NoError(t, err)
	}

	{ // Get the corresponding Public key for the serialnum
		pubkey, err := upldb.GetPublicKey(ctx, upID.ID)
		assert.NoError(t, err)
		pubbytes, err := x509.MarshalPKIXPublicKey(pubkey)
		assert.NoError(t, err)
		assert.EqualValues(t, upIDpubbytes, pubbytes)
	}
}
