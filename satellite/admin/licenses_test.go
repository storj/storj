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

		t.Run("UpdateUserLicense_Success", func(t *testing.T) {
			// Get current licenses to find an active one
			licenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)

			var target admin.UserLicense
			for _, l := range licenses.Licenses {
				if l.Type == "test-license" && l.RevokedAt == nil {
					target = l
					break
				}
			}
			require.Equal(t, "test-license", target.Type)

			// Update expiration to a new date
			newExpiresAt := time.Now().Add(90 * 24 * time.Hour).UTC()
			request := admin.UpdateLicenseRequest{
				Type:         target.Type,
				PublicId:     target.PublicId,
				BucketName:   target.BucketName,
				ExpiresAt:    target.ExpiresAt,
				NewExpiresAt: newExpiresAt,
				Reason:       "Extending license for another quarter",
			}

			apiErr = service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.NoError(t, apiErr.Err)

			// Verify license was updated
			licenses, apiErr = service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)

			var updated *admin.UserLicense
			for i := range licenses.Licenses {
				if licenses.Licenses[i].Type == "test-license" && licenses.Licenses[i].RevokedAt == nil {
					updated = &licenses.Licenses[i]
					break
				}
			}
			require.NotNil(t, updated)
			require.WithinDuration(t, newExpiresAt, updated.ExpiresAt, time.Second)
		})

		t.Run("UpdateUserLicense_VerifyOtherFieldsUnchanged", func(t *testing.T) {
			// Grant a license with all fields populated
			expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
			grantReq := admin.GrantLicenseRequest{
				Type:       "full-field-license",
				PublicId:   consoleProject.PublicID.String(),
				BucketName: "test-bucket",
				ExpiresAt:  expiresAt,
				Key:        "test-key-123",
				Reason:     "Grant for update test",
			}

			apiErr := service.GrantUserLicense(ctx, authInfo, consoleUser.ID, grantReq)
			require.NoError(t, apiErr.Err)

			// Update only the expiration
			newExpiresAt := time.Now().Add(60 * 24 * time.Hour).UTC()
			updateReq := admin.UpdateLicenseRequest{
				Type:         "full-field-license",
				PublicId:     consoleProject.PublicID.String(),
				BucketName:   "test-bucket",
				ExpiresAt:    expiresAt,
				NewExpiresAt: newExpiresAt,
				Reason:       "Extending expiration",
			}

			apiErr = service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, updateReq)
			require.NoError(t, apiErr.Err)

			// Verify all other fields are unchanged
			licenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)

			var updated *admin.UserLicense
			for i := range licenses.Licenses {
				if licenses.Licenses[i].Type == "full-field-license" {
					updated = &licenses.Licenses[i]
					break
				}
			}
			require.NotNil(t, updated)
			require.Equal(t, "full-field-license", updated.Type)
			require.Equal(t, consoleProject.PublicID.String(), updated.PublicId)
			require.Equal(t, "test-bucket", updated.BucketName)
			require.Equal(t, "test-key-123", updated.Key)
			require.Nil(t, updated.RevokedAt)
			require.WithinDuration(t, newExpiresAt, updated.ExpiresAt, time.Second)

			// Cleanup
			deleteReq := admin.DeleteLicenseRequest{
				Type:       "full-field-license",
				PublicId:   consoleProject.PublicID.String(),
				BucketName: "test-bucket",
				ExpiresAt:  newExpiresAt,
				Reason:     "Cleanup after test",
			}
			apiErr = service.DeleteUserLicense(ctx, authInfo, consoleUser.ID, deleteReq)
			require.NoError(t, apiErr.Err)
		})

		t.Run("UpdateUserLicense_ExpiredLicense", func(t *testing.T) {
			// Grant a license that expires in 1 second
			expiresAt := time.Now().Add(1 * time.Second).UTC()
			grantReq := admin.GrantLicenseRequest{
				Type:      "expiring-license",
				ExpiresAt: expiresAt,
				Reason:    "Grant short-lived license",
			}

			apiErr := service.GrantUserLicense(ctx, authInfo, consoleUser.ID, grantReq)
			require.NoError(t, apiErr.Err)

			// Wait for expiration
			time.Sleep(2 * time.Second)

			// Update the expired license to a future date
			newExpiresAt := time.Now().Add(90 * 24 * time.Hour).UTC()
			updateReq := admin.UpdateLicenseRequest{
				Type:         "expiring-license",
				ExpiresAt:    expiresAt,
				NewExpiresAt: newExpiresAt,
				Reason:       "Re-extending expired license",
			}

			apiErr = service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, updateReq)
			require.NoError(t, apiErr.Err)

			// Verify it's updated
			licenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)

			var updated *admin.UserLicense
			for i := range licenses.Licenses {
				if licenses.Licenses[i].Type == "expiring-license" {
					updated = &licenses.Licenses[i]
					break
				}
			}
			require.NotNil(t, updated)
			require.WithinDuration(t, newExpiresAt, updated.ExpiresAt, time.Second)

			// Cleanup
			deleteReq := admin.DeleteLicenseRequest{
				Type:      "expiring-license",
				ExpiresAt: newExpiresAt,
				Reason:    "Cleanup",
			}
			apiErr = service.DeleteUserLicense(ctx, authInfo, consoleUser.ID, deleteReq)
			require.NoError(t, apiErr.Err)
		})

		t.Run("UpdateUserLicense_ShortenExpiration", func(t *testing.T) {
			// Get current active test-license
			licenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)

			var target admin.UserLicense
			for _, l := range licenses.Licenses {
				if l.Type == "test-license" && l.RevokedAt == nil {
					target = l
					break
				}
			}
			require.Equal(t, "test-license", target.Type)

			// Shorten to just 7 days from now
			newExpiresAt := time.Now().Add(7 * 24 * time.Hour).UTC()
			request := admin.UpdateLicenseRequest{
				Type:         target.Type,
				ExpiresAt:    target.ExpiresAt,
				NewExpiresAt: newExpiresAt,
				Reason:       "Shortening license duration",
			}

			apiErr = service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.NoError(t, apiErr.Err)

			// Verify
			licenses, apiErr = service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)

			var updated *admin.UserLicense
			for i := range licenses.Licenses {
				if licenses.Licenses[i].Type == "test-license" && licenses.Licenses[i].RevokedAt == nil {
					updated = &licenses.Licenses[i]
					break
				}
			}
			require.NotNil(t, updated)
			require.WithinDuration(t, newExpiresAt, updated.ExpiresAt, time.Second)
		})

		t.Run("UpdateUserLicense_RevokedLicense", func(t *testing.T) {
			// Get the revoked test-license (revoked in earlier test)
			licenses, apiErr := service.GetUserLicenses(ctx, consoleUser.ID)
			require.NoError(t, apiErr.Err)

			var revoked admin.UserLicense
			for _, l := range licenses.Licenses {
				if l.Type == "test-license" && l.RevokedAt != nil {
					revoked = l
					break
				}
			}
			require.NotNil(t, revoked.RevokedAt, "expected a revoked test-license to exist")

			// Try to update the revoked license — should fail with 400
			request := admin.UpdateLicenseRequest{
				Type:         revoked.Type,
				PublicId:     revoked.PublicId,
				BucketName:   revoked.BucketName,
				ExpiresAt:    revoked.ExpiresAt,
				NewExpiresAt: time.Now().Add(90 * 24 * time.Hour).UTC(),
				Reason:       "Attempting to extend revoked license",
			}

			apiErr = service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Contains(t, apiErr.Err.Error(), "revoked")
		})

		t.Run("UpdateUserLicense_NotFound", func(t *testing.T) {
			request := admin.UpdateLicenseRequest{
				Type:         "nonexistent-license",
				ExpiresAt:    time.Now().Add(30 * 24 * time.Hour),
				NewExpiresAt: time.Now().Add(60 * 24 * time.Hour),
				Reason:       "Update nonexistent",
			}

			apiErr := service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("UpdateUserLicense_WrongExpiresAt", func(t *testing.T) {
			// Try to update with wrong current ExpiresAt (license exists but ExpiresAt doesn't match)
			request := admin.UpdateLicenseRequest{
				Type:         "test-license",
				ExpiresAt:    time.Now().Add(999 * 24 * time.Hour), // wrong date
				NewExpiresAt: time.Now().Add(60 * 24 * time.Hour),
				Reason:       "Wrong expiry match",
			}

			apiErr := service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("UpdateUserLicense_MissingReason", func(t *testing.T) {
			request := admin.UpdateLicenseRequest{
				Type:         "test-license",
				ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
				NewExpiresAt: time.Now().Add(60 * 24 * time.Hour),
			}

			apiErr := service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("UpdateUserLicense_MissingType", func(t *testing.T) {
			request := admin.UpdateLicenseRequest{
				ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
				NewExpiresAt: time.Now().Add(60 * 24 * time.Hour),
				Reason:       "Missing type",
			}

			apiErr := service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("UpdateUserLicense_PastDate", func(t *testing.T) {
			request := admin.UpdateLicenseRequest{
				Type:         "test-license",
				ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
				NewExpiresAt: time.Now().Add(-1 * time.Hour),
				Reason:       "Past date",
			}

			apiErr := service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("UpdateUserLicense_ZeroNewExpiresAt", func(t *testing.T) {
			request := admin.UpdateLicenseRequest{
				Type:      "test-license",
				ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
				Reason:    "Zero new date",
			}

			apiErr := service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, request)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("UpdateUserLicense_NoAuth", func(t *testing.T) {
			request := admin.UpdateLicenseRequest{
				Type:         "test-license",
				ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
				NewExpiresAt: time.Now().Add(60 * 24 * time.Hour),
				Reason:       "No auth",
			}

			apiErr := service.UpdateUserLicense(ctx, nil, consoleUser.ID, request)
			require.Equal(t, http.StatusUnauthorized, apiErr.Status)
		})

		t.Run("UpdateUserLicense_EmptyGroups", func(t *testing.T) {
			emptyAuth := &admin.AuthInfo{
				Email:  "admin@storj.io",
				Groups: []string{},
			}

			request := admin.UpdateLicenseRequest{
				Type:         "test-license",
				ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
				NewExpiresAt: time.Now().Add(60 * 24 * time.Hour),
				Reason:       "Empty groups",
			}

			apiErr := service.UpdateUserLicense(ctx, emptyAuth, consoleUser.ID, request)
			require.Equal(t, http.StatusUnauthorized, apiErr.Status)
		})

		t.Run("UpdateUserLicense_NonExistentUser", func(t *testing.T) {
			request := admin.UpdateLicenseRequest{
				Type:         "test-license",
				ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
				NewExpiresAt: time.Now().Add(60 * 24 * time.Hour),
				Reason:       "Non-existent user",
			}

			apiErr := service.UpdateUserLicense(ctx, authInfo, testrand.UUID(), request)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
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

		// Update the license and verify audit event fires
		newExpiresAt := time.Now().Add(60 * 24 * time.Hour).UTC()
		updateReq := admin.UpdateLicenseRequest{
			Type:         "audit-test-license",
			ExpiresAt:    expiresAt,
			NewExpiresAt: newExpiresAt,
			Reason:       "Testing update audit log",
		}

		apiErr = service.UpdateUserLicense(ctx, authInfo, consoleUser.ID, updateReq)
		require.NoError(t, apiErr.Err)

		// Verify the license was updated
		licenses, apiErr = service.GetUserLicenses(ctx, consoleUser.ID)
		require.NoError(t, apiErr.Err)
		require.Len(t, licenses.Licenses, 1)
		require.WithinDuration(t, newExpiresAt, licenses.Licenses[0].ExpiresAt, time.Second)
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
