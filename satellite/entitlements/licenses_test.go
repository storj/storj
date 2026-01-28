// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package entitlements_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestLicenseEntitlements(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		entSvc := entitlements.NewService(zaptest.NewLogger(t), db.Console().Entitlements())
		licenses := entSvc.Licenses()

		user, err := db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "test@storj.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)
		require.NotNil(t, user)

		userID := user.ID

		// Getting licenses for a user with no entitlements should return an error.
		got, err := licenses.Get(ctx, userID)
		require.NoError(t, err)
		require.Empty(t, got)

		now := time.Now().Truncate(time.Second)
		expectedPublicID := testrand.UUID().String()
		licensesToSet := entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{
				{
					Type:       "pro",
					PublicID:   expectedPublicID,
					BucketName: "my-bucket",
					ExpiresAt:  now.Add(30 * 24 * time.Hour),
				},
			},
		}

		err = licenses.Set(ctx, userID, licensesToSet)
		require.NoError(t, err)

		// Get licenses should return what we set.
		got, err = licenses.Get(ctx, userID)
		require.NoError(t, err)
		require.Len(t, got.Licenses, 1)
		require.Equal(t, "pro", got.Licenses[0].Type)
		require.Equal(t, expectedPublicID, got.Licenses[0].PublicID)
		require.Equal(t, "my-bucket", got.Licenses[0].BucketName)

		// Update licenses with additional entries.
		licensesToSet.Licenses = append(licensesToSet.Licenses, entitlements.AccountLicense{
			Type:       "enterprise",
			BucketName: "another-bucket",
		})

		err = licenses.Set(ctx, userID, licensesToSet)
		require.NoError(t, err)

		got, err = licenses.Get(ctx, userID)
		require.NoError(t, err)
		require.Len(t, got.Licenses, 2)
		require.Equal(t, "enterprise", got.Licenses[1].Type)
		require.Equal(t, "another-bucket", got.Licenses[1].BucketName)

		// Test with empty licenses list.
		err = licenses.Set(ctx, userID, entitlements.AccountLicenses{})
		require.NoError(t, err)

		got, err = licenses.Get(ctx, userID)
		require.NoError(t, err)
		require.Empty(t, got.Licenses)
	})
}
