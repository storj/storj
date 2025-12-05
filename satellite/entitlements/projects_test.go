// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package entitlements_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
)

func TestProjectEntitlements(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		entSvc := sat.API.Entitlements.Service
		projects := entSvc.Projects()

		proj, err := sat.DB.Console().Projects().Insert(ctx, &console.Project{Name: "ent-proj"})
		require.NoError(t, err)
		require.NotNil(t, proj)

		publicID := proj.PublicID

		err = projects.SetNewBucketPlacementsByPublicID(ctx, publicID, nil)
		require.Error(t, err)

		p1 := []storj.PlacementConstraint{0, 12}
		err = projects.SetNewBucketPlacementsByPublicID(ctx, publicID, p1)
		require.NoError(t, err)

		got, err := projects.GetByPublicID(ctx, publicID)
		require.NoError(t, err)
		require.ElementsMatch(t, p1, got.NewBucketPlacements)
		require.Empty(t, got.PlacementProductMappings)

		err = projects.SetPlacementProductMappingsByPublicID(ctx, publicID, nil)
		require.Error(t, err)

		m1 := entitlements.PlacementProductMappings{
			storj.DefaultPlacement: 1,
			12:                     2,
		}
		err = projects.SetPlacementProductMappingsByPublicID(ctx, publicID, m1)
		require.NoError(t, err)

		// Get should show BOTH fields (placements preserved).
		got, err = projects.GetByPublicID(ctx, publicID)
		require.NoError(t, err)
		require.ElementsMatch(t, p1, got.NewBucketPlacements)
		require.Equal(t, m1, got.PlacementProductMappings)

		// Update placements again; mappings must remain intact.
		p2 := []storj.PlacementConstraint{3}
		err = projects.SetNewBucketPlacementsByPublicID(ctx, publicID, p2)
		require.NoError(t, err)

		got, err = projects.GetByPublicID(ctx, publicID)
		require.NoError(t, err)
		require.ElementsMatch(t, p2, got.NewBucketPlacements)
		require.Equal(t, m1, got.PlacementProductMappings)

		// Update mappings again; placements must remain intact.
		m2 := entitlements.PlacementProductMappings{
			3: 3,
		}
		err = projects.SetPlacementProductMappingsByPublicID(ctx, publicID, m2)
		require.NoError(t, err)

		got, err = projects.GetByPublicID(ctx, publicID)
		require.NoError(t, err)
		require.ElementsMatch(t, p2, got.NewBucketPlacements)
		require.Equal(t, m2, got.PlacementProductMappings)
		require.Nil(t, got.ComputeAccessToken)

		// Test compute access token functionality.
		token1 := []byte("compute-token-123")
		err = projects.SetComputeAccessTokenByPublicID(ctx, publicID, token1)
		require.NoError(t, err)

		got, err = projects.GetByPublicID(ctx, publicID)
		require.NoError(t, err)
		require.ElementsMatch(t, p2, got.NewBucketPlacements)
		require.Equal(t, m2, got.PlacementProductMappings)
		require.Equal(t, token1, got.ComputeAccessToken)

		// Test setting token to empty byte slice.
		err = projects.SetComputeAccessTokenByPublicID(ctx, publicID, []byte{})
		require.NoError(t, err)

		got, err = projects.GetByPublicID(ctx, publicID)
		require.NoError(t, err)
		require.Nil(t, got.ComputeAccessToken)

		// Test setting token to nil.
		token2 := []byte("new-compute-token-456")
		err = projects.SetComputeAccessTokenByPublicID(ctx, publicID, token2)
		require.NoError(t, err)

		got, err = projects.GetByPublicID(ctx, publicID)
		require.NoError(t, err)
		require.Equal(t, token2, got.ComputeAccessToken)

		err = projects.SetComputeAccessTokenByPublicID(ctx, publicID, nil)
		require.NoError(t, err)

		got, err = projects.GetByPublicID(ctx, publicID)
		require.NoError(t, err)
		require.Nil(t, got.ComputeAccessToken)

		err = projects.DeleteByPublicID(ctx, publicID)
		require.NoError(t, err)

		_, err = projects.GetByPublicID(ctx, publicID)
		require.True(t, entitlements.ErrNotFound.Has(err))
	})
}
