// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestEntitlements(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		entDB := db.Console().Entitlements()

		now := time.Now()
		later := now.Add(5 * time.Minute)

		t.Run("Can't upsert nil entitlement", func(t *testing.T) {
			got, err := entDB.UpsertByScope(ctx, nil)
			require.Error(t, err)
			require.Nil(t, got)
		})

		t.Run("Can't upsert with empty scope", func(t *testing.T) {
			e := &entitlements.Entitlement{Scope: nil}

			got, err := entDB.UpsertByScope(ctx, e)
			require.Error(t, err)
			require.Nil(t, got)
		})

		scope := []byte("proj_id:" + testrand.UUID().String())
		newBucketPlacements := []storj.PlacementConstraint{storj.DefaultPlacement, 12}
		placementProductMappings := entitlements.PlacementProductMappings{
			storj.DefaultPlacement: 1,
			12:                     2,
		}

		t.Run("Upsert (create) entitlement", func(t *testing.T) {
			projectFeatures := entitlements.ProjectFeatures{
				NewBucketPlacements:      newBucketPlacements,
				PlacementProductMappings: placementProductMappings,
			}
			feats, err := json.Marshal(projectFeatures)
			require.NoError(t, err)

			got, err := entDB.UpsertByScope(ctx, &entitlements.Entitlement{
				Scope:     scope,
				Features:  feats,
				UpdatedAt: now,
			})
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, scope, got.Scope)
			require.False(t, got.CreatedAt.IsZero())
			require.WithinDuration(t, now, got.UpdatedAt, time.Minute)

			var gotFeats entitlements.ProjectFeatures
			err = json.Unmarshal(got.Features, &gotFeats)
			require.NoError(t, err)
			require.ElementsMatch(t, newBucketPlacements, gotFeats.NewBucketPlacements)
			require.Equal(t, placementProductMappings, gotFeats.PlacementProductMappings)
		})

		t.Run("Get by scope", func(t *testing.T) {
			got, err := entDB.GetByScope(ctx, scope)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, scope, got.Scope)

			var gotFeats entitlements.ProjectFeatures
			err = json.Unmarshal(got.Features, &gotFeats)
			require.NoError(t, err)
			require.ElementsMatch(t, newBucketPlacements, gotFeats.NewBucketPlacements)
			require.Equal(t, placementProductMappings, gotFeats.PlacementProductMappings)
		})

		t.Run("Upsert (update) preserves unrelated fields", func(t *testing.T) {
			newBucketPlacements1 := []storj.PlacementConstraint{3}
			projectFeatures := entitlements.ProjectFeatures{
				NewBucketPlacements:      newBucketPlacements1,
				PlacementProductMappings: placementProductMappings,
			}
			feats, err := json.Marshal(projectFeatures)
			require.NoError(t, err)

			e := &entitlements.Entitlement{
				Scope:     scope,
				Features:  feats,
				UpdatedAt: later,
			}

			got, err := entDB.UpsertByScope(ctx, e)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, scope, got.Scope)
			require.WithinDuration(t, later, got.UpdatedAt, time.Minute)

			var gotFeats entitlements.ProjectFeatures
			err = json.Unmarshal(got.Features, &gotFeats)
			require.NoError(t, err)
			require.ElementsMatch(t, newBucketPlacements1, gotFeats.NewBucketPlacements)
			require.Equal(t, placementProductMappings, gotFeats.PlacementProductMappings)

			refetched, err := entDB.GetByScope(ctx, scope)
			require.NoError(t, err)
			require.NotNil(t, refetched)

			var refetchedFeats entitlements.ProjectFeatures
			err = json.Unmarshal(refetched.Features, &refetchedFeats)
			require.NoError(t, err)
			require.ElementsMatch(t, newBucketPlacements1, refetchedFeats.NewBucketPlacements)
			require.Equal(t, placementProductMappings, refetchedFeats.PlacementProductMappings)
		})

		t.Run("Delete by scope", func(t *testing.T) {
			err := entDB.DeleteByScope(ctx, scope)
			require.NoError(t, err)

			got, err := entDB.GetByScope(ctx, scope)
			require.True(t, entitlements.ErrNotFound.Has(err))
			require.Nil(t, got)
		})
	})
}
