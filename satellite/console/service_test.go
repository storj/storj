// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
)

func TestService(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]
			service := sat.API.Console.Service

			up1Pro1, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
			require.NoError(t, err)
			up2Pro1, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[1].Projects[0].ID)
			require.NoError(t, err)

			up2User, err := sat.API.DB.Console().Users().Get(ctx, up2Pro1.OwnerID)
			require.NoError(t, err)

			require.NotEqual(t, up1Pro1.ID, up2Pro1.ID)
			require.NotEqual(t, up1Pro1.OwnerID, up2Pro1.OwnerID)

			authCtx1, err := sat.AuthenticatedContext(ctx, up1Pro1.OwnerID)
			require.NoError(t, err)

			authCtx2, err := sat.AuthenticatedContext(ctx, up2Pro1.OwnerID)
			require.NoError(t, err)

			t.Run("TestGetProject", func(t *testing.T) {
				// Getting own project details should work
				project, err := service.GetProject(authCtx1, up1Pro1.ID)
				require.NoError(t, err)
				require.Equal(t, up1Pro1.ID, project.ID)

				// Getting someone else project details should not work
				project, err = service.GetProject(authCtx1, up2Pro1.ID)
				require.Error(t, err)
				require.Nil(t, project)
			})

			t.Run("TestUpdateProject", func(t *testing.T) {
				// Updating own project should work
				updatedPro, err := service.UpdateProject(authCtx1, up1Pro1.ID, "newName", "TestUpdate")
				require.NoError(t, err)
				require.NotEqual(t, up1Pro1.Name, updatedPro.Name)

				// Updating someone else project details should not work
				updatedPro, err = service.UpdateProject(authCtx1, up2Pro1.ID, "newName", "TestUpdate")
				require.Error(t, err)
				require.Nil(t, updatedPro)
			})

			t.Run("TestAddProjectMembers", func(t *testing.T) {
				// Adding members to own project should work
				addedUsers, err := service.AddProjectMembers(authCtx1, up1Pro1.ID, []string{up2User.Email})
				require.NoError(t, err)
				require.Len(t, addedUsers, 1)
				require.Contains(t, addedUsers, up2User)

				// Adding members to someone else project should not work
				addedUsers, err = service.AddProjectMembers(authCtx1, up2Pro1.ID, []string{up2User.Email})
				require.Error(t, err)
				require.Nil(t, addedUsers)
			})

			t.Run("TestGetProjectMembers", func(t *testing.T) {
				// Getting the project members of an own project that one is a part of should work
				userPage, err := service.GetProjectMembers(authCtx1, up1Pro1.ID, console.ProjectMembersCursor{Page: 1, Limit: 10})
				require.NoError(t, err)
				require.Len(t, userPage.ProjectMembers, 2)

				// Getting the project members of a foreign project that one is a part of should work
				userPage, err = service.GetProjectMembers(authCtx2, up1Pro1.ID, console.ProjectMembersCursor{Page: 1, Limit: 10})
				require.NoError(t, err)
				require.Len(t, userPage.ProjectMembers, 2)

				// Getting the project members of a foreign project that one is not a part of should not work
				userPage, err = service.GetProjectMembers(authCtx1, up2Pro1.ID, console.ProjectMembersCursor{Page: 1, Limit: 10})
				require.Error(t, err)
				require.Nil(t, userPage)
			})

			t.Run("TestDeleteProjectMembers", func(t *testing.T) {
				// Deleting project members of an own project should work
				err := service.DeleteProjectMembers(authCtx1, up1Pro1.ID, []string{up2User.Email})
				require.NoError(t, err)

				// Deleting Project members of someone else project should not work
				err = service.DeleteProjectMembers(authCtx1, up2Pro1.ID, []string{up2User.Email})
				require.Error(t, err)
			})

			t.Run("TestDeleteProject", func(t *testing.T) {
				// Deleting the own project should not work before deleting the API-Key
				err := service.DeleteProject(authCtx1, up1Pro1.ID)
				require.Error(t, err)

				keys, err := service.GetAPIKeys(authCtx1, up1Pro1.ID, console.APIKeyCursor{Page: 1, Limit: 10})
				require.NoError(t, err)
				require.Len(t, keys.APIKeys, 1)

				err = service.DeleteAPIKeys(authCtx1, []uuid.UUID{keys.APIKeys[0].ID})
				require.NoError(t, err)

				// Deleting the own project should now work
				err = service.DeleteProject(authCtx1, up1Pro1.ID)
				require.NoError(t, err)

				// Deleting someone else project should not work
				err = service.DeleteProject(authCtx1, up2Pro1.ID)
				require.Error(t, err)

				err = planet.Uplinks[1].CreateBucket(ctx, sat, "testbucket")
				require.NoError(t, err)

				// deleting a project with a bucket should fail
				err = service.DeleteProject(authCtx2, up2Pro1.ID)
				require.Error(t, err)
				require.Equal(t, "service error: project usage error: some buckets still exist", err.Error())
			})

			t.Run("TestChangeEmail", func(t *testing.T) {
				const newEmail = "newEmail@example.com"

				err = service.ChangeEmail(authCtx2, newEmail)
				require.NoError(t, err)

				userWithUpdatedEmail, err := service.GetUserByEmail(authCtx2, newEmail)
				require.NoError(t, err)
				require.Equal(t, newEmail, userWithUpdatedEmail.Email)

				err = service.ChangeEmail(authCtx2, newEmail)
				require.Error(t, err)
			})

			t.Run("TestGetAllBucketNames", func(t *testing.T) {
				bucket1 := storj.Bucket{
					ID:        testrand.UUID(),
					Name:      "testBucket1",
					ProjectID: up2Pro1.ID,
				}

				bucket2 := storj.Bucket{
					ID:        testrand.UUID(),
					Name:      "testBucket2",
					ProjectID: up2Pro1.ID,
				}

				_, err := sat.DB.Buckets().CreateBucket(authCtx2, bucket1)
				require.NoError(t, err)

				_, err = sat.DB.Buckets().CreateBucket(authCtx2, bucket2)
				require.NoError(t, err)

				bucketNames, err := service.GetAllBucketNames(authCtx2, up2Pro1.ID)
				require.NoError(t, err)
				require.Equal(t, bucket1.Name, bucketNames[0])
				require.Equal(t, bucket2.Name, bucketNames[1])

				// Getting someone else buckets should not work
				bucketsForUnauthorizedUser, err := service.GetAllBucketNames(authCtx1, up2Pro1.ID)
				require.Error(t, err)
				require.Nil(t, bucketsForUnauthorizedUser)
			})

			t.Run("TestDeleteAPIKeyByNameAndProjectID", func(t *testing.T) {
				secret, err := macaroon.NewSecret()
				require.NoError(t, err)

				key, err := macaroon.NewAPIKey(secret)
				require.NoError(t, err)

				apikey := console.APIKeyInfo{
					Name:      "test",
					ProjectID: up2Pro1.ID,
					Secret:    secret,
				}

				createdKey, err := sat.DB.Console().APIKeys().Create(ctx, key.Head(), apikey)
				require.NoError(t, err)

				info, err := sat.DB.Console().APIKeys().Get(ctx, createdKey.ID)
				require.NoError(t, err)
				require.NotNil(t, info)

				// Deleting someone else api keys should not work
				err = service.DeleteAPIKeyByNameAndProjectID(authCtx1, apikey.Name, up2Pro1.ID)
				require.Error(t, err)

				err = service.DeleteAPIKeyByNameAndProjectID(authCtx2, apikey.Name, up2Pro1.ID)
				require.NoError(t, err)

				info, err = sat.DB.Console().APIKeys().Get(ctx, createdKey.ID)
				require.Error(t, err)
				require.Nil(t, info)
			})
		})
}
