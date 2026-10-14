// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/api"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	backoffice "storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/console"
)

func TestInspectAndRevokeAccess(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			SatelliteDBOptions: testplanet.SatelliteDBDisableCaches,
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.UserGroupsRoleAdmin = []string{"admin"}
				config.Admin.UserGroupsRoleViewer = []string{"viewer"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()

		t.Run("empty access string", func(t *testing.T) {
			result, apiErr := service.InspectAccess(ctx, backoffice.AccessInspectRequest{Access: ""})
			require.Nil(t, result)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Error(t, apiErr.Err)
		})

		t.Run("invalid access string", func(t *testing.T) {
			result, apiErr := service.InspectAccess(ctx, backoffice.AccessInspectRequest{Access: "notvalid"})
			require.Nil(t, result)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Error(t, apiErr.Err)
		})

		t.Run("valid access grant", func(t *testing.T) {
			user, err := consoleDB.Users().Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				FullName:     "Test User",
				Email:        "testinspect@storj.io",
				Status:       console.Active,
				PasswordHash: make([]byte, 0),
			})
			require.NoError(t, err)

			project, err := sat.AddProject(ctx, user.ID, "Inspect Test Project")
			require.NoError(t, err)

			apiKey, err := sat.CreateAPIKey(ctx, project.ID, user.ID, macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			encAccess := grant.NewEncryptionAccessWithDefaultKey(&storj.Key{})
			encAccess.SetDefaultPathCipher(storj.EncAESGCM)

			grantAccess := grant.Access{
				SatelliteAddress: sat.URL(),
				APIKey:           apiKey,
				EncAccess:        encAccess,
			}

			serialized, err := grantAccess.Serialize()
			require.NoError(t, err)

			result, apiErr := service.InspectAccess(ctx, backoffice.AccessInspectRequest{Access: serialized})
			require.NoError(t, apiErr.Err)
			require.NotNil(t, result)

			require.Equal(t, sat.URL(), result.SatelliteAddr)
			require.Equal(t, project.PublicID.String(), result.PublicProjectID)
			require.Equal(t, user.Email, result.ProjectOwnerEmail)
			require.Equal(t, user.ID.String(), result.CreatorID)
			require.Equal(t, "AES128-GCM", result.DefaultPathCipher)
			require.False(t, result.Revoked)
			require.NotEmpty(t, result.APIKey)
		})

		t.Run("revoke access", func(t *testing.T) {
			user, err := consoleDB.Users().Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				FullName:     "Revoke User",
				Email:        "revokeaccess@storj.io",
				Status:       console.Active,
				PasswordHash: make([]byte, 0),
			})
			require.NoError(t, err)

			project, err := sat.AddProject(ctx, user.ID, "Revoke Test Project")
			require.NoError(t, err)

			apiKey, err := sat.CreateAPIKey(ctx, project.ID, user.ID, macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			encAccess := grant.NewEncryptionAccessWithDefaultKey(&storj.Key{})
			encAccess.SetDefaultPathCipher(storj.EncAESGCM)

			grantAccess := grant.Access{
				SatelliteAddress: sat.URL(),
				APIKey:           apiKey,
				EncAccess:        encAccess,
			}

			serialized, err := grantAccess.Serialize()
			require.NoError(t, err)

			result, apiErr := service.InspectAccess(ctx, backoffice.AccessInspectRequest{Access: serialized})
			require.NoError(t, apiErr.Err)
			require.NotNil(t, result)
			require.False(t, result.Revoked)
			require.NotEmpty(t, result.Macaroon.Tail)

			authInfo := &backoffice.AuthInfo{Groups: []string{"admin"}}
			reason := "test-reason"

			t.Run("missing api key", func(t *testing.T) {
				httpErr := service.RevokeAccess(ctx, authInfo, backoffice.AccessRevokeRequest{
					APIKey: "",
					Reason: reason,
				})
				require.Equal(t, http.StatusBadRequest, httpErr.Status)
				require.Error(t, httpErr.Err)
			})

			t.Run("invalid api key", func(t *testing.T) {
				httpErr := service.RevokeAccess(ctx, authInfo, backoffice.AccessRevokeRequest{
					APIKey: "notvalid",
					Reason: reason,
				})
				require.Equal(t, http.StatusBadRequest, httpErr.Status)
				require.Error(t, httpErr.Err)
			})

			t.Run("missing reason", func(t *testing.T) {
				httpErr := service.RevokeAccess(ctx, authInfo, backoffice.AccessRevokeRequest{
					APIKey: result.APIKey,
					Reason: "",
				})
				require.Equal(t, http.StatusBadRequest, httpErr.Status)
				require.Error(t, httpErr.Err)
			})

			t.Run("success", func(t *testing.T) {
				httpErr := service.RevokeAccess(ctx, authInfo, backoffice.AccessRevokeRequest{
					APIKey: result.APIKey,
					Reason: reason,
				})
				require.NoError(t, httpErr.Err)

				result, apiErr = service.InspectAccess(ctx, backoffice.AccessInspectRequest{Access: serialized})
				require.NoError(t, apiErr.Err)
				require.NotNil(t, result)
				require.True(t, result.Revoked)
			})
		})

		t.Run("valid access with caveats", func(t *testing.T) {
			user, err := consoleDB.Users().Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				FullName:     "Caveat User",
				Email:        "caveat@storj.io",
				Status:       console.Active,
				PasswordHash: make([]byte, 0),
			})
			require.NoError(t, err)

			project, err := sat.AddProject(ctx, user.ID, "Caveat Test Project")
			require.NoError(t, err)

			apiKey, err := sat.CreateAPIKey(ctx, project.ID, user.ID, macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			restrictedKey, err := apiKey.Restrict(macaroon.Caveat{DisallowWrites: true})
			require.NoError(t, err)

			encAccess := grant.NewEncryptionAccessWithDefaultKey(&storj.Key{})
			encAccess.SetDefaultPathCipher(storj.EncAESGCM)

			grantAccess := grant.Access{
				SatelliteAddress: sat.URL(),
				APIKey:           restrictedKey,
				EncAccess:        encAccess,
			}

			serialized, err := grantAccess.Serialize()
			require.NoError(t, err)

			result, apiErr := service.InspectAccess(ctx, backoffice.AccessInspectRequest{Access: serialized})
			require.NoError(t, apiErr.Err)
			require.NotNil(t, result)
			require.GreaterOrEqual(t, len(result.Macaroon.Caveats), 1)
		})
	})
}

func TestInspectAccessViaAuthService(t *testing.T) {
	var (
		mu             sync.Mutex
		responseGrant  string
		responseStatus = http.StatusOK
	)

	const (
		authToken = "test-auth-service-token"
		// Exactly 28 characters — triggers the S3 key ID path in InspectAccess.
		keyID = "AAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	)

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+authToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		mu.Lock()
		accessGrant := responseGrant
		status := responseStatus
		mu.Unlock()

		if status != http.StatusOK {
			http.Error(w, "service error", status)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_grant": accessGrant})
	}))
	t.Cleanup(authServer.Close)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			SatelliteDBOptions: testplanet.SatelliteDBDisableCaches,
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.AuthServiceAccessEndpoint = authServer.URL
				config.Admin.AuthServiceAccessToken = authToken
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()

		user, err := consoleDB.Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "Auth Service User",
			Email:        "authservice@storj.io",
			Status:       console.Active,
			PasswordHash: make([]byte, 0),
		})
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "Auth Service Test Project")
		require.NoError(t, err)

		apiKey, err := sat.CreateAPIKey(ctx, project.ID, user.ID, macaroon.APIKeyVersionMin)
		require.NoError(t, err)

		encAccess := grant.NewEncryptionAccessWithDefaultKey(&storj.Key{})
		encAccess.SetDefaultPathCipher(storj.EncAESGCM)

		grantAccess := grant.Access{
			SatelliteAddress: sat.URL(),
			APIKey:           apiKey,
			EncAccess:        encAccess,
		}

		serialized, err := grantAccess.Serialize()
		require.NoError(t, err)

		t.Run("success", func(t *testing.T) {
			mu.Lock()
			responseGrant = serialized
			responseStatus = http.StatusOK
			mu.Unlock()

			result, apiErr := service.InspectAccess(ctx, backoffice.AccessInspectRequest{Access: keyID})
			require.NoError(t, apiErr.Err)
			require.NotNil(t, result)
			require.Equal(t, sat.URL(), result.SatelliteAddr)
			require.Equal(t, user.Email, result.ProjectOwnerEmail)
		})

		t.Run("auth service error", func(t *testing.T) {
			mu.Lock()
			responseStatus = http.StatusInternalServerError
			mu.Unlock()

			result, apiErr := service.InspectAccess(ctx, backoffice.AccessInspectRequest{Access: keyID})
			require.Nil(t, result)
			require.Equal(t, http.StatusInternalServerError, apiErr.Status)
			require.Error(t, apiErr.Err)
		})

		t.Run("empty access grant from auth service", func(t *testing.T) {
			mu.Lock()
			responseGrant = ""
			responseStatus = http.StatusOK
			mu.Unlock()

			result, apiErr := service.InspectAccess(ctx, backoffice.AccessInspectRequest{Access: keyID})
			require.Nil(t, result)
			require.Equal(t, http.StatusInternalServerError, apiErr.Status)
			require.Error(t, apiErr.Err)
		})
	})
}

func TestRevokeAccessTenantScoping(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			SatelliteDBOptions: testplanet.SatelliteDBDisableCaches,
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.UserGroupsRoleAdmin = []string{"admin"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()

		tenantA := "tenant-a"
		tenantB := "tenant-b"

		// createAccess creates a user in the given tenant with a project and an access
		// grant, and returns the grant and its inspection result.
		createAccess := func(email string, tenantID *string) (string, *backoffice.AccessInspectResult) {
			user, err := consoleDB.Users().Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				FullName:     "Revoke Tenant User",
				Email:        email,
				Status:       console.Active,
				PasswordHash: make([]byte, 0),
				TenantID:     tenantID,
			})
			require.NoError(t, err)

			project, err := sat.AddProject(ctx, user.ID, "Revoke Tenant Project")
			require.NoError(t, err)

			apiKey, err := sat.CreateAPIKey(ctx, project.ID, user.ID, macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			encAccess := grant.NewEncryptionAccessWithDefaultKey(&storj.Key{})
			encAccess.SetDefaultPathCipher(storj.EncAESGCM)

			serialized, err := (&grant.Access{
				SatelliteAddress: sat.URL(),
				APIKey:           apiKey,
				EncAccess:        encAccess,
			}).Serialize()
			require.NoError(t, err)

			result, apiErr := service.InspectAccess(ctx, backoffice.AccessInspectRequest{Access: serialized})
			require.NoError(t, apiErr.Err)
			require.False(t, result.Revoked)
			return serialized, result
		}

		authInfo := &backoffice.AuthInfo{Email: "admin@storj.io", Groups: []string{"admin"}}
		revokeKey := func(apiKey string) api.HTTPError {
			return service.RevokeAccess(ctx, authInfo, backoffice.AccessRevokeRequest{
				APIKey: apiKey,
				Reason: "test",
			})
		}
		revoke := func(result *backoffice.AccessInspectResult) api.HTTPError {
			return revokeKey(result.APIKey)
		}
		isRevoked := func(serialized string) bool {
			result, apiErr := service.InspectAccess(ctx, backoffice.AccessInspectRequest{Access: serialized})
			require.NoError(t, apiErr.Err)
			return result.Revoked
		}

		t.Cleanup(func() { service.TestSetTenantID(nil) })

		t.Run("tenant-scoped admin: other tenant access returns 404", func(t *testing.T) {
			serialized, result := createAccess("revoke-tenant-b@example.com", &tenantB)

			service.TestSetTenantID(&tenantA)
			apiErr := revoke(result)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
			require.False(t, isRevoked(serialized))
		})

		t.Run("tenant-scoped admin: access without tenant returns 404", func(t *testing.T) {
			serialized, result := createAccess("revoke-no-tenant@example.com", nil)

			service.TestSetTenantID(&tenantA)
			apiErr := revoke(result)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
			require.False(t, isRevoked(serialized))
		})

		t.Run("tenant-scoped admin: own key with other tenant's tail is rejected", func(t *testing.T) {
			_, ownResult := createAccess("revoke-forge-a@example.com", &tenantA)
			otherSerialized, otherResult := createAccess("revoke-forge-b@example.com", &tenantB)

			ownKey, err := macaroon.ParseAPIKey(ownResult.APIKey)
			require.NoError(t, err)

			// Keep the head of the own key, which passes the tenant check, but swap in the
			// tail of the other tenant's key, which is what would get revoked.
			forgedRaw := bytes.Replace(ownKey.SerializeRaw(), ownResult.Macaroon.Tail, otherResult.Macaroon.Tail, 1)
			require.NotEqual(t, ownKey.SerializeRaw(), forgedRaw)
			forged, err := macaroon.ParseRawAPIKey(forgedRaw)
			require.NoError(t, err)
			require.Equal(t, otherResult.Macaroon.Tail, forged.Tail())

			service.TestSetTenantID(&tenantA)
			apiErr := revokeKey(forged.Serialize())
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.False(t, isRevoked(otherSerialized))
		})

		t.Run("tenant-scoped admin: own tenant access is revoked", func(t *testing.T) {
			serialized, result := createAccess("revoke-tenant-a@example.com", &tenantA)

			service.TestSetTenantID(&tenantA)
			apiErr := revoke(result)
			require.NoError(t, apiErr.Err)
			require.True(t, isRevoked(serialized))
		})

		t.Run("general admin revokes access of any tenant", func(t *testing.T) {
			serialized, result := createAccess("revoke-general-b@example.com", &tenantB)

			service.TestSetTenantID(nil)
			apiErr := revoke(result)
			require.NoError(t, apiErr.Err)
			require.True(t, isRevoked(serialized))
		})
	})
}
