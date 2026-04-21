// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	backoffice "storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/console"
)

func TestInspectAccess(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
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
