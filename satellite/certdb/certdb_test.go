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
	{ //uplink testing variables
		upID, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)
		upIDpubbytes, err := x509.MarshalPKIXPublicKey(upID.Leaf.PublicKey)
		require.NoError(t, err)

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

		{ // Get the corresponding Public key for the serialnum
			pubkey, err := upldb.GetPublicKeys(ctx, upID.ID)
			assert.NoError(t, err)
			pubbytes, err := x509.MarshalPKIXPublicKey(pubkey[0])
			assert.NoError(t, err)
			assert.EqualValues(t, upIDpubbytes, pubbytes)
		}
	}

	{ //storagenode testing variables
		sn1ID, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)
		sn1IDpubbytes, err := x509.MarshalPKIXPublicKey(sn1ID.Leaf.PublicKey)
		require.NoError(t, err)

		{ // New entry
			err := upldb.SavePublicKey(ctx, sn1ID.ID, sn1ID.Leaf.PublicKey)
			assert.NoError(t, err)
		}

		sn2ID, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)
		sn2IDpubbytes, err := x509.MarshalPKIXPublicKey(sn2ID.Leaf.PublicKey)
		require.NoError(t, err)

		{ // add another key for the same storagenode ID
			err := upldb.SavePublicKey(ctx, sn1ID.ID, sn2ID.Leaf.PublicKey)
			assert.NoError(t, err)
		}

		{ // add another key for the same storagenode ID, this the latest key
			err := upldb.SavePublicKey(ctx, sn1ID.ID, sn2ID.Leaf.PublicKey)
			assert.NoError(t, err)
		}

		{ // Get the corresponding Public key for the serialnum
			// test to return one key but the latest of the keys
			pkey, err := upldb.GetPublicKey(ctx, sn1ID.ID)
			assert.NoError(t, err)
			pbytes, err := x509.MarshalPKIXPublicKey(pkey)
			assert.NoError(t, err)
			assert.EqualValues(t, sn2IDpubbytes, pbytes)

			// test all the keys for a given ID
			pubkey, err := upldb.GetPublicKeys(ctx, sn1ID.ID)
			assert.NoError(t, err)
			assert.Equal(t, 2, len(pubkey))
			pubbytes, err := x509.MarshalPKIXPublicKey(pubkey[0])
			assert.NoError(t, err)
			assert.EqualValues(t, sn2IDpubbytes, pubbytes)
			pubbytes, err = x509.MarshalPKIXPublicKey(pubkey[1])
			assert.NoError(t, err)
			assert.EqualValues(t, sn1IDpubbytes, pubbytes)
		}
	}
}
