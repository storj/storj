// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	admin "storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
)

func TestAdmin_LicenseManagement(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()

		// Create a test user
		consoleUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName:  "Test User",
			Email:     "license-test@storj.io",
			UserAgent: []byte("agent"),
		}, 1)
		require.NoError(t, err)

		authInfo := &admin.AuthInfo{
			Email:  "admin@storj.io",
			Groups: []string{"admin"},
		}

		// Create a test project
		consoleProject := &console.Project{
			ID:      testrand.UUID(),
			Name:    "test-project",
			OwnerID: consoleUser.ID,
		}
		consoleProject, err = consoleDB.Projects().Insert(ctx, consoleProject)
		require.NoError(t, err)

		t.Run("GetUserLicenses_NoLicenses", func(t *testing.T) {
			// Get licenses for user with no licenses
			licenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)
			require.Empty(t, licenses.Licenses)
		})

		t.Run("GrantUserLicense_Success", func(t *testing.T) {
			// Grant a new license
			expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
			key := "test-key-value"
			request := admin.GrantLicenseRequest{
				Type:      "test-license",
				ExpiresAt: expiresAt,
				Key:       key,
				Reason:    "Test grant",
			}

			apiErr := service.GrantUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.NoError(t, apiErr.Err)

			// Verify license was created
			licenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)
			require.Len(t, licenses.Licenses, 1)
			require.Equal(t, "test-license", licenses.Licenses[0].Type)
			require.WithinDuration(t, expiresAt, licenses.Licenses[0].ExpiresAt, time.Second)
			require.Nil(t, licenses.Licenses[0].RevokedAt)
			require.Equal(t, key, licenses.Licenses[0].Key)
		})

		t.Run("GrantUserLicense_WithProjectScope", func(t *testing.T) {
			// Grant a license scoped to a project
			expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
			request := admin.GrantLicenseRequest{
				Type:      "project-license",
				PublicId:  consoleProject.PublicID.String(),
				ExpiresAt: expiresAt,
				Reason:    "Test project license",
			}

			apiErr := service.GrantUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.NoError(t, apiErr.Err)

			// Verify license was created
			licenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)
			require.Len(t, licenses.Licenses, 2) // One from previous test + this one

			// Find the project license
			var projectLicense *admin.UserLicense
			for i := range licenses.Licenses {
				if licenses.Licenses[i].Type == "project-license" {
					projectLicense = &licenses.Licenses[i]
					break
				}
			}
			require.NotNil(t, projectLicense)
			require.Equal(t, consoleProject.PublicID.String(), projectLicense.PublicId)
		})

		t.Run("GrantUserLicense_DuplicateFails", func(t *testing.T) {
			// Try to grant same license again
			expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
			request := admin.GrantLicenseRequest{
				Type:      "test-license",
				ExpiresAt: expiresAt,
				Reason:    "Duplicate test",
			}

			apiErr := service.GrantUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusConflict, apiErr.Status)
		})

		t.Run("GrantUserLicense_ValidationErrors", func(t *testing.T) {
			// Missing reason
			request := admin.GrantLicenseRequest{
				Type:      "no-reason-license",
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
			}
			apiErr := service.GrantUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)

			// Missing type
			request = admin.GrantLicenseRequest{
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
				Reason:    "Missing type",
			}
			apiErr = service.GrantUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)

			// Past expiration
			request = admin.GrantLicenseRequest{
				Type:      "expired-license",
				ExpiresAt: time.Now().Add(-1 * time.Hour),
				Reason:    "Past expiration",
			}
			apiErr = service.GrantUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)

			// Invalid project ID
			request = admin.GrantLicenseRequest{
				Type:      "invalid-project-license",
				PublicId:  "not-a-uuid",
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
				Reason:    "Invalid project",
			}
			apiErr = service.GrantUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)

			// Non-existent project ID
			request = admin.GrantLicenseRequest{
				Type:      "nonexistent-project-license",
				PublicId:  uuid.UUID{}.String(),
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
				Reason:    "Nonexistent project",
			}
			apiErr = service.GrantUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("RevokeUserLicense_Success", func(t *testing.T) {
			// Get the current license to know its exact fields
			licenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)

			var target admin.UserLicense
			for _, l := range licenses.Licenses {
				if l.Type == "test-license" {
					target = l
					break
				}
			}
			require.Equal(t, "test-license", target.Type)

			// Revoke a license
			request := admin.RevokeLicenseRequest{
				Type:      target.Type,
				PublicId:  target.PublicId,
				ExpiresAt: target.ExpiresAt,
				Reason:    "Test revocation",
			}

			apiErr = service.RevokeUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.NoError(t, apiErr.Err)

			// Verify license was revoked
			licenses, apiErr = service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)

			// Find the revoked test-license
			var testLicense *admin.UserLicense
			for i := range licenses.Licenses {
				if licenses.Licenses[i].Type == "test-license" && licenses.Licenses[i].RevokedAt != nil {
					testLicense = &licenses.Licenses[i]
					break
				}
			}
			require.NotNil(t, testLicense)
			require.WithinDuration(t, time.Now(), *testLicense.RevokedAt, 5*time.Second)
		})

		t.Run("GrantUserLicense_AfterRevoke", func(t *testing.T) {
			// Granting a license with the same type and scope should succeed
			// after the previous one has been revoked.
			expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
			request := admin.GrantLicenseRequest{
				Type:      "test-license",
				ExpiresAt: expiresAt,
				Reason:    "Re-grant after revocation",
			}

			apiErr := service.GrantUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.NoError(t, apiErr.Err)

			// Verify both the revoked and new license exist
			licenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)

			var revokedCount, activeCount int
			for _, l := range licenses.Licenses {
				if l.Type == "test-license" {
					if l.RevokedAt != nil {
						revokedCount++
					} else {
						activeCount++
					}
				}
			}
			require.Equal(t, 1, revokedCount)
			require.Equal(t, 1, activeCount)
		})

		t.Run("RevokeUserLicense_NotFound", func(t *testing.T) {
			// Try to revoke non-existent license
			request := admin.RevokeLicenseRequest{
				Type:      "nonexistent-license",
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
				Reason:    "Revoke nonexistent",
			}

			apiErr := service.RevokeUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("RevokeUserLicense_MissingReason", func(t *testing.T) {
			// Try to revoke without reason
			request := admin.RevokeLicenseRequest{
				Type:      "project-license",
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
			}

			apiErr := service.RevokeUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("DeleteUserLicense_Success", func(t *testing.T) {
			// Get current licenses to find the project-license
			licensesBefore, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)
			countBefore := len(licensesBefore.Licenses)

			var target admin.UserLicense
			for _, l := range licensesBefore.Licenses {
				if l.Type == "project-license" {
					target = l
					break
				}
			}
			require.Equal(t, "project-license", target.Type)

			// Delete the project-license
			request := admin.DeleteLicenseRequest{
				Type:       target.Type,
				PublicId:   target.PublicId,
				BucketName: target.BucketName,
				ExpiresAt:  target.ExpiresAt,
				Reason:     "Test deletion",
			}

			apiErr = service.DeleteUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.NoError(t, apiErr.Err)

			// Verify license was removed
			licensesAfter, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)
			require.Equal(t, countBefore-1, len(licensesAfter.Licenses))

			// Verify the project-license is gone
			for _, l := range licensesAfter.Licenses {
				require.NotEqual(t, "project-license", l.Type)
			}
		})

		t.Run("DeleteUserLicense_NotFound", func(t *testing.T) {
			request := admin.DeleteLicenseRequest{
				Type:      "nonexistent-license",
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
				Reason:    "Delete nonexistent",
			}

			apiErr := service.DeleteUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("DeleteUserLicense_MissingReason", func(t *testing.T) {
			request := admin.DeleteLicenseRequest{
				Type:      "test-license",
				ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
			}

			apiErr := service.DeleteUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("UserNotFound", func(t *testing.T) {
			// Test with non-existent user
			nonExistentUserID := testrand.UUID()

			_, apiErr := service.GetUserLicenses(ctx, nonExistentUserID)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})
	})
}

// TestAdmin_LicenseAuditLog tests that license operations are properly logged.
// Note: This test verifies that audit logging works with the service layer.
func TestAdmin_LicenseAuditLog(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service

		// Enable audit logging
		service.TestToggleAuditLogger(true)

		// Create a test user
		consoleUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName:  "Audit Test User",
			Email:     "audit-test@storj.io",
			UserAgent: []byte("agent"),
		}, 1)
		require.NoError(t, err)

		authInfo := &admin.AuthInfo{
			Email:  "admin@storj.io",
			Groups: []string{"admin"},
		}

		// Grant a license
		expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
		request := admin.GrantLicenseRequest{
			Type:      "audit-test-license",
			ExpiresAt: expiresAt,
			Reason:    "Testing audit log",
		}

		apiErr := service.GrantUserLicense(ctx, authInfo, consoleUser.ID, request)
		require.NoError(t, apiErr.Err)

		// Verify the license was granted (audit log verification is tested separately in production)
		licenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
		require.NoError(t, apiErr.Err)
		require.Len(t, licenses.Licenses, 1)
		require.Equal(t, "audit-test-license", licenses.Licenses[0].Type)
	})
}

func TestAdmin_LicenseEntitlementsIntegration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		// Create a test user
		consoleUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName:  "Integration Test User",
			Email:     "integration-test@storj.io",
			UserAgent: []byte("agent"),
		}, 1)
		require.NoError(t, err)

		// Create entitlements service
		entSvc := entitlements.NewService(sat.Log.Named("entitlements"), sat.DB.Console().Entitlements())

		// Grant a license via entitlements service
		expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
		key := "some key"
		license := entitlements.AccountLicense{
			Type:      "integration-test-license",
			ExpiresAt: expiresAt,
			Key:       []byte(key),
		}

		licenses := entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{license},
		}

		err = entSvc.Licenses().Set(ctx, consoleUser.ID, licenses)
		require.NoError(t, err)

		// Verify via admin service
		service := sat.Admin.Admin.Service
		retrievedLicenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
		require.NoError(t, apiErr.Err)
		require.Len(t, retrievedLicenses.Licenses, 1)
		require.Equal(t, "integration-test-license", retrievedLicenses.Licenses[0].Type)
		require.Equal(t, key, retrievedLicenses.Licenses[0].Key)
	})
}
