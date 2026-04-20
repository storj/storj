// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
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

			t.Run("missing tail", func(t *testing.T) {
				httpErr := service.RevokeAccess(ctx, authInfo, backoffice.AccessRevokeRequest{
					Tail:     nil,
					APIKeyID: result.APIKeyID,
					Reason:   reason,
				})
				require.Equal(t, http.StatusBadRequest, httpErr.Status)
				require.Error(t, httpErr.Err)
			})

			t.Run("missing api key id", func(t *testing.T) {
				httpErr := service.RevokeAccess(ctx, authInfo, backoffice.AccessRevokeRequest{
					Tail:     result.Macaroon.Tail,
					APIKeyID: "",
					Reason:   reason,
				})
				require.Equal(t, http.StatusBadRequest, httpErr.Status)
				require.Error(t, httpErr.Err)
			})

			t.Run("success", func(t *testing.T) {
				httpErr := service.RevokeAccess(ctx, authInfo, backoffice.AccessRevokeRequest{
					Tail:     result.Macaroon.Tail,
					APIKeyID: result.APIKeyID,
					Reason:   reason,
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
