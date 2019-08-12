// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package certdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
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

func testDatabase(ctx context.Context, t *testing.T, snCerts certdb.DB) {
	{ //testing variables
		snID, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)
		pi := snID.PeerIdentity()
		expectedpubBytes, err := pkcrypto.PublicKeyToPEM(pi.CA.PublicKey)
		require.NoError(t, err)

		{ // New entry
			err := snCerts.Set(ctx, snID.ID, pi)
			assert.NoError(t, err)
		}

		{ // Get the corresponding Public key for the serialnum
			snpi, err := snCerts.Get(ctx, snID.ID)
			assert.NoError(t, err)
			pubBytes, err := pkcrypto.PublicKeyToPEM(snpi.CA.PublicKey)
			assert.NoError(t, err)
			assert.EqualValues(t, expectedpubBytes, pubBytes)
		}
	}

	{ //storagenode testing variables
		sn1FI, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)
		sn1PI := sn1FI.PeerIdentity()

		{ // New entry
			err := snCerts.Set(ctx, sn1PI.ID, sn1PI)
			assert.NoError(t, err)
		}
		sn2FI, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)
		sn2PI := sn2FI.PeerIdentity()
		sn2PIpubBytes, err := pkcrypto.PublicKeyToPEM(sn2PI.CA.PublicKey)
		require.NoError(t, err)

		{ // adding two different pubkeys for same storagenode
			{ // add a key for to storagenode ID
				err := snCerts.Set(ctx, sn2PI.ID, sn1PI)
				assert.NoError(t, err)
			}
			{ // add another key for the same storagenode ID, this the latest key
				// as this is written later than the previous one by few seconds
				err := snCerts.Set(ctx, sn2PI.ID, sn2PI)
				assert.NoError(t, err)
			}
			{ // already existing public key, just return nil
				err := snCerts.Set(ctx, sn1PI.ID, sn1PI)
				assert.NoError(t, err)
			}
		}

		{ // Get the corresponding Public key for the ID
			// test to return one key but the latest of the keys
			pkey, err := snCerts.Get(ctx, sn2PI.ID)
			assert.NoError(t, err)
			pbytes, err := pkcrypto.PublicKeyToPEM(pkey.CA.PublicKey)
			require.NoError(t, err)
			assert.EqualValues(t, sn2PIpubBytes, pbytes)
		}

		{ // Get all the corresponding Public key for the IDs
			var PIDs []*identity.PeerIdentity
			var NIDs []storj.NodeID
			for i := 0; i < 10; i++ {
				fid, err := testidentity.NewTestIdentity(ctx)
				require.NoError(t, err)
				PIDs = append(PIDs, fid.PeerIdentity())
				NIDs = append(NIDs, fid.PeerIdentity().ID)
				err = snCerts.Set(ctx, fid.PeerIdentity().ID, fid.PeerIdentity())
				assert.NoError(t, err)
			}
			pkey, err := snCerts.BatchGet(ctx, NIDs)
			assert.NoError(t, err)
			assert.NotNil(t, pkey)
			assert.Equal(t, 10, len(pkey))
		}
	}
}
