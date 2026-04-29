// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestDeleteAPIKeyByNameAndProjectID(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		newUser := console.CreateUser{
			FullName:  "test_name",
			ShortName: "",
			Email:     "apikeytest@test.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "apikeytest")
		require.NoError(t, err)

		secret, err := macaroon.NewSecret()
		require.NoError(t, err)

		key, err := macaroon.NewAPIKey(secret)
		require.NoError(t, err)

		apikey := console.APIKeyInfo{
			Name:      "test",
			ProjectID: project.ID,
			Secret:    secret,
		}

		deleteTestFunc := func(endpointSuffix string) func(t *testing.T) {
			return func(t *testing.T) {
				created, err := sat.DB.Console().APIKeys().Create(ctx, key.Head(), apikey)
				require.NoError(t, err)

				endpoint := "api-keys/delete-by-name?name=" + apikey.Name
				_, status, err := doRequestWithAuth(ctx, sat, user, http.MethodDelete, endpoint+endpointSuffix, nil)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, status)

				keyAfterDelete, err := sat.DB.Console().APIKeys().Get(ctx, created.ID)
				require.Error(t, err)
				require.Nil(t, keyAfterDelete)
			}
		}

		t.Run("delete by name and projectID", deleteTestFunc("&projectID="+project.ID.String()))
		t.Run("delete by name and publicID", deleteTestFunc("&publicID="+project.PublicID.String()))
	})
}

func TestGetAllAPIKeyNamesByProjectID(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		newUser := console.CreateUser{
			FullName:  "test_name",
			ShortName: "",
			Email:     "apikeytest1@test.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "apikeytest")
		require.NoError(t, err)

		secret, err := macaroon.NewSecret()
		require.NoError(t, err)

		key, err := macaroon.NewAPIKey(secret)
		require.NoError(t, err)

		apikey := console.APIKeyInfo{
			Name:      "test",
			ProjectID: project.ID,
			Secret:    secret,
		}

		secret1, err := macaroon.NewSecret()
		require.NoError(t, err)

		key1, err := macaroon.NewAPIKey(secret1)
		require.NoError(t, err)

		apikey1 := console.APIKeyInfo{
			Name:      "test1",
			ProjectID: project.ID,
			Secret:    secret1,
		}

		created, err := sat.DB.Console().APIKeys().Create(ctx, key.Head(), apikey)
		require.NoError(t, err)

		created1, err := sat.DB.Console().APIKeys().Create(ctx, key1.Head(), apikey1)
		require.NoError(t, err)

		endpoint := "api-keys/api-key-names?projectID=" + project.ID.String()
		body, status, err := doRequestWithAuth(ctx, sat, user, http.MethodGet, endpoint, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		var output []string

		err = json.Unmarshal(body, &output)
		require.NoError(t, err)

		require.Equal(t, 2, len(output))
		require.Equal(t, created.Name, output[0])
		require.Equal(t, created1.Name, output[1])
	})
}

func TestCreateAuditableAPIKey(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		keysDB := sat.DB.Console().APIKeys()
		service := sat.API.Console.Service

		newUser := console.CreateUser{
			FullName: "test_name",
			Email:    "apikeytest@example.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "test")
		require.NoError(t, err)

		endpoint := "api-keys/create/" + project.PublicID.String()
		buf := bytes.NewBufferString("testName")

		_, status, err := doRequestWithAuth(ctx, sat, user, http.MethodPost, endpoint, buf)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		info, err := keysDB.GetByNameAndProjectID(ctx, "testName", project.ID)
		require.NoError(t, err)
		require.NotNil(t, info)
		// object lock and eventing are enabled by default.
		require.Equal(t, macaroon.APIKeyVersionObjectLock|macaroon.APIKeyVersionEventing, info.Version)
		require.False(t, info.Version.SupportsAuditability())

		service.TestSetAuditableAPIKeyProjects(map[string]struct{}{project.PublicID.String(): {}})
		buf = bytes.NewBufferString("testName1")

		_, status, err = doRequestWithAuth(ctx, sat, user, http.MethodPost, endpoint, buf)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		info, err = keysDB.GetByNameAndProjectID(ctx, "testName1", project.ID)
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Equal(t, macaroon.APIKeyVersionObjectLock|macaroon.APIKeyVersionAuditable|macaroon.APIKeyVersionEventing, info.Version)
		require.True(t, info.Version.SupportsAuditability())
	})
}

func TestCreateEventingAPIKey(t *testing.T) {
	enabledProjects := eventingconfig.ProjectSet{}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.BucketEventing.Projects = enabledProjects
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		keysDB := sat.DB.Console().APIKeys()

		newUser := console.CreateUser{
			FullName: "test_name",
			Email:    "apikeytest@example.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, user.ID, "test")
		require.NoError(t, err)

		endpoint := "api-keys/create/" + project.PublicID.String()
		buf := bytes.NewBufferString("testName")

		_, status, err := doRequestWithAuth(ctx, t, sat, user, http.MethodPost, endpoint, buf)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		info, err := keysDB.GetByNameAndProjectID(ctx, "testName", project.ID)
		require.NoError(t, err)
		require.NotNil(t, info)
		// object lock is enabled by default.
		require.Equal(t, macaroon.APIKeyVersionObjectLock, info.Version)
		require.False(t, info.Version.SupportsEventing())

		// Enable eventing for the test project
		enabledProjects[project.ID] = struct{}{}
		buf = bytes.NewBufferString("testName1")

		_, status, err = doRequestWithAuth(ctx, t, sat, user, http.MethodPost, endpoint, buf)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		info, err = keysDB.GetByNameAndProjectID(ctx, "testName1", project.ID)
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Equal(t, macaroon.APIKeyVersionObjectLock|macaroon.APIKeyVersionEventing, info.Version)
		require.True(t, info.Version.SupportsEventing())
	})
}

func TestUpdateAPIKey(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		keysDB := sat.DB.Console().APIKeys()
		service := sat.API.Console.Service

		owner, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Owner Name",
			Email:    "updatekey_owner@test.test",
		}, 1)
		require.NoError(t, err)

		member, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Member Name",
			Email:    "updatekey_member@test.test",
		}, 1)
		require.NoError(t, err)

		nonMember, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Non Member Name",
			Email:    "updatekey_nonmember@test.test",
		}, 1)
		require.NoError(t, err)

		project, err := sat.AddProject(ctx, owner.ID, "Update Key Project")
		require.NoError(t, err)

		ownerCtx, err := sat.UserContext(ctx, owner.ID)
		require.NoError(t, err)
		memberCtx, err := sat.UserContext(ctx, member.ID)
		require.NoError(t, err)

		_, err = service.AddProjectMembers(ownerCtx, project.ID, []string{member.Email})
		require.NoError(t, err)

		ownerKey, _, err := service.CreateAPIKey(ownerCtx, project.ID, "owner's key", macaroon.APIKeyVersionMin)
		require.NoError(t, err)
		require.NotNil(t, ownerKey)

		memberKey, _, err := service.CreateAPIKey(memberCtx, project.ID, "member's key", macaroon.APIKeyVersionMin)
		require.NoError(t, err)
		require.NotNil(t, memberKey)

		t.Run("successful update by owner", func(t *testing.T) {
			newName := "owner's key updated"
			payload := map[string]string{"name": newName}
			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			endpoint := "api-keys/update/" + ownerKey.ID.String()
			body, status, err := doRequestWithAuth(ctx, t, sat, owner, http.MethodPut, endpoint, bytes.NewBuffer(payloadBytes))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)

			var updatedKey console.APIKeyInfo
			err = json.Unmarshal(body, &updatedKey)
			require.NoError(t, err)
			require.Equal(t, newName, updatedKey.Name)
			require.Equal(t, ownerKey.ID, updatedKey.ID)

			dbKey, err := keysDB.Get(ctx, ownerKey.ID)
			require.NoError(t, err)
			require.Equal(t, newName, dbKey.Name)
		})

		t.Run("successful update by member of own key", func(t *testing.T) {
			newName := "member's key updated"
			payload := map[string]string{"name": newName}
			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			endpoint := "api-keys/update/" + memberKey.ID.String()
			body, status, err := doRequestWithAuth(ctx, t, sat, member, http.MethodPut, endpoint, bytes.NewBuffer(payloadBytes))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)

			var updatedKey console.APIKeyInfo
			err = json.Unmarshal(body, &updatedKey)
			require.NoError(t, err)
			require.Equal(t, newName, updatedKey.Name)

			dbKey, err := keysDB.Get(ctx, memberKey.ID)
			require.NoError(t, err)
			require.Equal(t, newName, dbKey.Name)
		})

		t.Run("member cannot update owner's key", func(t *testing.T) {
			newName := "hacked name"
			payload := map[string]string{"name": newName}
			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			endpoint := "api-keys/update/" + ownerKey.ID.String()
			_, status, err := doRequestWithAuth(ctx, t, sat, member, http.MethodPut, endpoint, bytes.NewBuffer(payloadBytes))
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, status)

			dbKey, err := keysDB.Get(ctx, ownerKey.ID)
			require.NoError(t, err)
			require.NotEqual(t, newName, dbKey.Name)
		})

		t.Run("non-member cannot update any key", func(t *testing.T) {
			newName := "hacked name"
			payload := map[string]string{"name": newName}
			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			endpoint := "api-keys/update/" + ownerKey.ID.String()
			_, status, err := doRequestWithAuth(ctx, t, sat, nonMember, http.MethodPut, endpoint, bytes.NewBuffer(payloadBytes))
			require.NoError(t, err)
			require.Equal(t, http.StatusUnauthorized, status)
		})

		t.Run("admin can update any key", func(t *testing.T) {
			_, err = service.UpdateProjectMemberRole(ownerCtx, member.ID, project.ID, console.RoleAdmin)
			require.NoError(t, err)

			newName := "owner's key updated by admin"
			payload := map[string]string{"name": newName}
			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			endpoint := "api-keys/update/" + ownerKey.ID.String()
			body, status, err := doRequestWithAuth(ctx, t, sat, member, http.MethodPut, endpoint, bytes.NewBuffer(payloadBytes))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)

			var updatedKey console.APIKeyInfo
			err = json.Unmarshal(body, &updatedKey)
			require.NoError(t, err)
			require.Equal(t, newName, updatedKey.Name)
		})

		t.Run("duplicate name validation", func(t *testing.T) {
			_, _, err := service.CreateAPIKey(ownerCtx, project.ID, "other key", macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			payload := map[string]string{"name": "other key"}
			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			endpoint := "api-keys/update/" + ownerKey.ID.String()
			_, status, err := doRequestWithAuth(ctx, t, sat, owner, http.MethodPut, endpoint, bytes.NewBuffer(payloadBytes))
			require.NoError(t, err)
			require.Equal(t, http.StatusConflict, status)

			dbKey, err := keysDB.Get(ctx, ownerKey.ID)
			require.NoError(t, err)
			require.NotEqual(t, "other key", dbKey.Name)
		})

		t.Run("can update to same name", func(t *testing.T) {
			dbKey, err := keysDB.Get(ctx, memberKey.ID)
			require.NoError(t, err)
			currentName := dbKey.Name

			payload := map[string]string{"name": currentName}
			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			endpoint := "api-keys/update/" + memberKey.ID.String()
			body, status, err := doRequestWithAuth(ctx, t, sat, member, http.MethodPut, endpoint, bytes.NewBuffer(payloadBytes))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)

			var updatedKey console.APIKeyInfo
			err = json.Unmarshal(body, &updatedKey)
			require.NoError(t, err)
			require.Equal(t, currentName, updatedKey.Name)
		})
	})
}
