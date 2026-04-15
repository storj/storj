// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestRegistrationTokens(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		regTokens := db.Console().RegistrationTokens()

		t.Run("Create and GetBySecret", func(t *testing.T) {
			token, err := regTokens.Create(ctx, 5)
			require.NoError(t, err)
			require.NotNil(t, token)
			require.False(t, token.Secret.IsZero())
			require.Equal(t, 5, token.ProjectLimit)
			require.Nil(t, token.OwnerID)
			require.Nil(t, token.StorageLimit)
			require.Nil(t, token.BandwidthLimit)
			require.Nil(t, token.SegmentLimit)
			require.Nil(t, token.ExpiresAt)
			require.Nil(t, token.UserKind)
			require.Nil(t, token.Partner)
			require.False(t, token.CreatedAt.IsZero())

			got, err := regTokens.GetBySecret(ctx, token.Secret)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, token.Secret, got.Secret)
			require.Equal(t, token.ProjectLimit, got.ProjectLimit)
			require.Nil(t, got.OwnerID)
		})

		t.Run("CreateWithLimits", func(t *testing.T) {
			storageLimit := int64(1000000)
			bandwidthLimit := int64(2000000)
			segmentLimit := int64(100)
			expiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond).UTC()
			userKind := console.PaidUser
			partner := "test-partner"

			token, err := regTokens.CreateWithLimits(ctx, console.CreateRegistrationTokenParams{
				ProjectLimit:   10,
				StorageLimit:   &storageLimit,
				BandwidthLimit: &bandwidthLimit,
				SegmentLimit:   &segmentLimit,
				ExpiresAt:      &expiresAt,
				UserKind:       &userKind,
				Partner:        &partner,
			})
			require.NoError(t, err)
			require.NotNil(t, token)
			require.False(t, token.Secret.IsZero())
			require.Equal(t, 10, token.ProjectLimit)
			require.NotNil(t, token.StorageLimit)
			require.Equal(t, storageLimit, *token.StorageLimit)
			require.NotNil(t, token.BandwidthLimit)
			require.Equal(t, bandwidthLimit, *token.BandwidthLimit)
			require.NotNil(t, token.SegmentLimit)
			require.Equal(t, segmentLimit, *token.SegmentLimit)
			require.NotNil(t, token.ExpiresAt)
			require.WithinDuration(t, expiresAt, *token.ExpiresAt, time.Second)
			require.NotNil(t, token.UserKind)
			require.Equal(t, userKind, *token.UserKind)
			require.NotNil(t, token.Partner)
			require.Equal(t, partner, *token.Partner)

			got, err := regTokens.GetBySecret(ctx, token.Secret)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, storageLimit, *got.StorageLimit)
			require.Equal(t, bandwidthLimit, *got.BandwidthLimit)
			require.Equal(t, segmentLimit, *got.SegmentLimit)
			require.WithinDuration(t, expiresAt, *got.ExpiresAt, time.Second)
			require.NotNil(t, got.UserKind)
			require.Equal(t, userKind, *got.UserKind)
			require.NotNil(t, got.Partner)
			require.Equal(t, partner, *got.Partner)
		})

		t.Run("CreateWithLimits nil optionals", func(t *testing.T) {
			token, err := regTokens.CreateWithLimits(ctx, console.CreateRegistrationTokenParams{
				ProjectLimit: 3,
			})
			require.NoError(t, err)
			require.NotNil(t, token)
			require.Equal(t, 3, token.ProjectLimit)
			require.Nil(t, token.StorageLimit)
			require.Nil(t, token.BandwidthLimit)
			require.Nil(t, token.SegmentLimit)
			require.Nil(t, token.ExpiresAt)
			require.Nil(t, token.UserKind)
			require.Nil(t, token.Partner)
		})

		t.Run("CreateWithLimits UserKind variants", func(t *testing.T) {
			for _, kind := range console.UserKinds {
				kind := kind
				token, err := regTokens.CreateWithLimits(ctx, console.CreateRegistrationTokenParams{
					ProjectLimit: 1,
					UserKind:     &kind,
				})
				require.NoError(t, err)
				require.NotNil(t, token)
				require.NotNil(t, token.UserKind)
				require.Equal(t, kind, *token.UserKind)

				got, err := regTokens.GetBySecret(ctx, token.Secret)
				require.NoError(t, err)
				require.NotNil(t, got.UserKind)
				require.Equal(t, kind, *got.UserKind)
			}
		})

		t.Run("UpdateOwner and GetByOwnerID", func(t *testing.T) {
			token, err := regTokens.Create(ctx, 2)
			require.NoError(t, err)

			ownerID := testrand.UUID()
			err = regTokens.UpdateOwner(ctx, token.Secret, ownerID)
			require.NoError(t, err)

			got, err := regTokens.GetByOwnerID(ctx, ownerID)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, token.Secret, got.Secret)
			require.NotNil(t, got.OwnerID)
			require.Equal(t, ownerID, *got.OwnerID)

			gotBySecret, err := regTokens.GetBySecret(ctx, token.Secret)
			require.NoError(t, err)
			require.NotNil(t, gotBySecret.OwnerID)
			require.Equal(t, ownerID, *gotBySecret.OwnerID)
		})
	})
}
