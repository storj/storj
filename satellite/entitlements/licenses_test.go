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

		now := time.Now().UTC().Truncate(time.Second)
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

func TestLicenses_GetActive(t *testing.T) {
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
		publicID := testrand.UUID()
		now := time.Now().UTC().Truncate(time.Second)

		t.Run("no licenses", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{})
			require.NoError(t, err)
			require.Empty(t, active)
		})

		// Set up various licenses for testing.
		expectedLicenses := []entitlements.AccountLicense{
			{
				Type:      "pro",
				ExpiresAt: now.Add(30 * 24 * time.Hour),
			},
			{
				Type:      "enterprise",
				PublicID:  publicID.String(),
				ExpiresAt: now.Add(60 * 24 * time.Hour),
			},
			{
				Type:       "basic",
				PublicID:   publicID.String(),
				BucketName: "specific-bucket",
				ExpiresAt:  now.Add(90 * 24 * time.Hour),
			},
			{
				Type:      "expired",
				ExpiresAt: now.Add(-24 * time.Hour),
			},
			{
				Type:      "revoked",
				ExpiresAt: now.Add(30 * 24 * time.Hour),
				RevokedAt: now.Add(-24 * time.Hour),
			},
			{
				Type:      "future-revoked",
				ExpiresAt: now.Add(30 * 24 * time.Hour),
				RevokedAt: now.Add(24 * time.Hour),
			},
		}

		require.NoError(t, licenses.Set(ctx, userID, entitlements.AccountLicenses{
			Licenses: expectedLicenses,
		}))

		t.Run("get all active without filters", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{})
			require.NoError(t, err)
			require.Len(t, active, len(expectedLicenses))
			require.ElementsMatch(t, expectedLicenses, active)
		})

		t.Run("filter by time - exclude expired", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				Now: &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[1],
				expectedLicenses[2],
				expectedLicenses[5],
			}, active)
		})

		t.Run("filter by license type", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				LicenseType: "pro",
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
			}, active)
		})

		t.Run("filter by license type and time", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				LicenseType: "expired",
				Now:         &now,
			})
			require.NoError(t, err)
			require.Empty(t, active)
		})

		t.Run("global license matches all projects", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				PublicID: publicID,
				Now:      &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[1],
				expectedLicenses[2],
				expectedLicenses[5],
			}, active)
		})

		t.Run("project-specific license", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				LicenseType: "enterprise",
				PublicID:    publicID,
				Now:         &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[1],
			}, active)
		})

		t.Run("bucket-specific license", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				PublicID:   publicID,
				BucketName: "specific-bucket",
				Now:        &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[1],
				expectedLicenses[2],
				expectedLicenses[5],
			}, active)
		})

		t.Run("non-matching project", func(t *testing.T) {
			otherPublicID := testrand.UUID()
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				PublicID: otherPublicID,
				Now:      &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[5],
			}, active)
		})

		t.Run("non-matching bucket", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				PublicID:   publicID,
				BucketName: "other-bucket",
				Now:        &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[1],
				expectedLicenses[5],
			}, active)
		})

		t.Run("future time excludes future revocations", func(t *testing.T) {
			futureTime := now.Add(48 * time.Hour)
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				Now: &futureTime,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[1],
				expectedLicenses[2],
			}, active)
		})

		t.Run("non-existent user", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, testrand.UUID(), entitlements.GetActiveOptions{})
			require.NoError(t, err)
			require.Empty(t, active)
		})
	})
}
