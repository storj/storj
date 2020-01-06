// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestPeerIdentities(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		encode := identity.EncodePeerIdentity

		idents := db.PeerIdentities()

		{ // basic tests
			ca, err := testidentity.NewTestCA(ctx)
			require.NoError(t, err)

			leafFirst, err := ca.NewIdentity()
			require.NoError(t, err)

			leafSecond, err := ca.NewIdentity()
			require.NoError(t, err)

			// sanity check
			require.Equal(t, leafFirst.ID, leafSecond.ID)

			{ // add entry
				err := idents.Set(ctx, leafFirst.ID, leafFirst.PeerIdentity())
				require.NoError(t, err)
			}

			{ // get the entry
				got, err := idents.Get(ctx, leafFirst.ID)
				require.NoError(t, err)
				require.Equal(t, encode(leafFirst.PeerIdentity()), encode(got))
			}

			{ // update entry
				err := idents.Set(ctx, leafSecond.ID, leafSecond.PeerIdentity())
				require.NoError(t, err)
			}

			{ // get the entry
				got, err := idents.Get(ctx, leafFirst.ID)
				require.NoError(t, err)
				require.Equal(t, encode(leafSecond.PeerIdentity()), encode(got))
			}
		}

		{ // get multiple with invalid
			list := make(map[storj.NodeID]*identity.PeerIdentity)
			ids := storj.NodeIDList{}
			savedIDs := make(map[storj.NodeID]bool)
			unsavedIDs := make(map[storj.NodeID]bool)

			for i := 0; i < 10; i++ {
				ident := testidentity.MustPregeneratedSignedIdentity(i, storj.LatestIDVersion())
				list[ident.ID] = ident.PeerIdentity()
				ids = append(ids, ident.ID)

				// only place half in the table
				if i%2 == 0 {
					unsavedIDs[ident.ID] = true
					continue
				}
				err := idents.Set(ctx, ident.ID, ident.PeerIdentity())
				require.NoError(t, err)
				savedIDs[ident.ID] = true
			}

			got, err := idents.BatchGet(ctx, ids)
			require.NoError(t, err)
			require.Len(t, got, len(savedIDs))
			for _, gotIdent := range got {
				require.Equal(t, encode(list[gotIdent.ID]), encode(gotIdent))
				require.True(t, savedIDs[gotIdent.ID])
				require.False(t, unsavedIDs[gotIdent.ID])
			}
		}
	})
}
