// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package certdb_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/pkcrypto"
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
		pi := upID.PeerIdentity()
		upIDpubbytes, err := pkcrypto.PublicKeyToPEM(pi.CA.PublicKey)
		require.NoError(t, err)

		{ // New entry
			err := upldb.Set(ctx, upID.ID, pi)
			assert.NoError(t, err)
		}

		{ // Get the corresponding Public key for the serialnum
			uplpi, err := upldb.Get(ctx, upID.ID)
			assert.NoError(t, err)
			pubbytes, err := pkcrypto.PublicKeyToPEM(uplpi.CA.PublicKey)
			assert.NoError(t, err)
			assert.EqualValues(t, upIDpubbytes, pubbytes)
		}
	}

	{ //storagenode testing variables
		sn1FI, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)
		sn1PI := sn1FI.PeerIdentity()

		{ // New entry
			err := upldb.Set(ctx, sn1PI.ID, sn1PI)
			assert.NoError(t, err)
		}
		sn2FI, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)
		sn2PI := sn2FI.PeerIdentity()
		sn2PIpubbytes, err := pkcrypto.PublicKeyToPEM(sn2PI.CA.PublicKey)
		require.NoError(t, err)

		{ // adding two different pubkeys for same storagnode
			{ // add a key for to storagenode ID
				err := upldb.Set(ctx, sn2PI.ID, sn1PI)
				assert.NoError(t, err)
			}
			time.Sleep(5)
			{ // add another key for the same storagenode ID, this the latest key
				// as this is written later than the previous one by few seconds
				err := upldb.Set(ctx, sn2PI.ID, sn2PI)
				assert.NoError(t, err)
			}
			{ // already existing public key, just return nil
				err := upldb.Set(ctx, sn1PI.ID, sn1PI)
				assert.NoError(t, err)
			}
		}

		{ // Get the corresponding Public key for the ID
			// test to return one key but the latest of the keys
			pkey, err := upldb.Get(ctx, sn2PI.ID)
			assert.NoError(t, err)
			pbytes, err := pkcrypto.PublicKeyToPEM(pkey.CA.PublicKey)
			require.NoError(t, err)
			assert.EqualValues(t, sn2PIpubbytes, pbytes)
		}
	}
}
