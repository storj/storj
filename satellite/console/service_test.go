// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
)

func TestService(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
	},
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
				updatedName := "newName"
				updatedDescription := "newDescription"
				updatedStorageLimit := memory.Size(100)
				updatedBandwidthLimit := memory.Size(100)

				// user should be in free tier
				user, err := service.GetUser(ctx, up1Pro1.OwnerID)
				require.NoError(t, err)
				require.False(t, user.PaidTier)
				// get context
				authCtx1, err := sat.AuthenticatedContext(ctx, user.ID)
				require.NoError(t, err)
				// add a credit card to put the user in the paid tier
				err = service.Payments().AddCreditCard(authCtx1, "test-cc-token")
				require.NoError(t, err)
				// update auth ctx
				authCtx1, err = sat.AuthenticatedContext(ctx, user.ID)
				require.NoError(t, err)

				// Updating own project should work
				updatedProject, err := service.UpdateProject(authCtx1, up1Pro1.ID, console.ProjectInfo{
					Name:           updatedName,
					Description:    updatedDescription,
					StorageLimit:   updatedStorageLimit,
					BandwidthLimit: updatedBandwidthLimit,
				})
				require.NoError(t, err)
				require.NotEqual(t, up1Pro1.Name, updatedProject.Name)
				require.Equal(t, updatedName, updatedProject.Name)
				require.NotEqual(t, up1Pro1.Description, updatedProject.Description)
				require.Equal(t, updatedDescription, updatedProject.Description)
				require.NotEqual(t, *up1Pro1.StorageLimit, *updatedProject.StorageLimit)
				require.Equal(t, updatedStorageLimit, *updatedProject.StorageLimit)
				require.NotEqual(t, *up1Pro1.BandwidthLimit, *updatedProject.BandwidthLimit)
				require.Equal(t, updatedBandwidthLimit, *updatedProject.BandwidthLimit)

				// Updating someone else project details should not work
				updatedProject, err = service.UpdateProject(authCtx1, up2Pro1.ID, console.ProjectInfo{
					Name:           "newName",
					Description:    "TestUpdate",
					StorageLimit:   memory.Size(100),
					BandwidthLimit: memory.Size(100),
				})
				require.Error(t, err)
				require.Nil(t, updatedProject)

				// attempting to update a project with bandwidth or storage limits set to 0 should fail
				size0 := new(memory.Size)
				*size0 = 0
				size100 := new(memory.Size)
				*size100 = memory.Size(100)

				up1Pro1.StorageLimit = size0
				err = sat.DB.Console().Projects().Update(ctx, up1Pro1)
				require.NoError(t, err)

				updateInfo := console.ProjectInfo{
					Name:           "a b c",
					Description:    "1 2 3",
					StorageLimit:   memory.Size(123),
					BandwidthLimit: memory.Size(123),
				}
				updatedProject, err = service.UpdateProject(authCtx1, up1Pro1.ID, updateInfo)
				require.Error(t, err)
				require.Nil(t, updatedProject)

				up1Pro1.StorageLimit = size100
				up1Pro1.BandwidthLimit = size0

				err = sat.DB.Console().Projects().Update(ctx, up1Pro1)
				require.NoError(t, err)

				updatedProject, err = service.UpdateProject(authCtx1, up1Pro1.ID, updateInfo)
				require.Error(t, err)
				require.Nil(t, updatedProject)

				up1Pro1.StorageLimit = size100
				up1Pro1.BandwidthLimit = size100
				err = sat.DB.Console().Projects().Update(ctx, up1Pro1)
				require.NoError(t, err)

				updatedProject, err = service.UpdateProject(authCtx1, up1Pro1.ID, updateInfo)
				require.NoError(t, err)
				require.Equal(t, updateInfo.Name, updatedProject.Name)
				require.Equal(t, updateInfo.Description, updatedProject.Description)
				require.NotNil(t, updatedProject.StorageLimit)
				require.NotNil(t, updatedProject.BandwidthLimit)
				require.Equal(t, updateInfo.StorageLimit, *updatedProject.StorageLimit)
				require.Equal(t, updateInfo.BandwidthLimit, *updatedProject.BandwidthLimit)

				project, err := service.GetProject(authCtx1, up1Pro1.ID)
				require.NoError(t, err)
				require.Equal(t, updateInfo.StorageLimit, *project.StorageLimit)
				require.Equal(t, updateInfo.BandwidthLimit, *project.BandwidthLimit)
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
				require.Equal(t, "console service: project usage: some buckets still exist", err.Error())
			})

			t.Run("TestChangeEmail", func(t *testing.T) {
				const newEmail = "newEmail@example.com"

				err = service.ChangeEmail(authCtx2, newEmail)
				require.NoError(t, err)

				user, _, err := service.GetUserByEmailWithUnverified(authCtx2, newEmail)
				require.NoError(t, err)
				require.Equal(t, newEmail, user.Email)

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

				_, err := sat.API.Buckets.Service.CreateBucket(authCtx2, bucket1)
				require.NoError(t, err)

				_, err = sat.API.Buckets.Service.CreateBucket(authCtx2, bucket2)
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

func TestPaidTier(t *testing.T) {
	usageConfig := console.UsageLimitsConfig{
		Storage: console.StorageLimitConfig{
			Free: memory.GB,
			Paid: memory.TB,
		},
		Bandwidth: console.BandwidthLimitConfig{
			Free: 2 * memory.GB,
			Paid: 2 * memory.TB,
		},
		Segment: console.SegmentLimitConfig{
			Free: 10,
			Paid: 50,
		},
		Project: console.ProjectLimitConfig{
			Free: 1,
			Paid: 3,
		},
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.UsageLimits = usageConfig
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		// project should have free tier usage limits
		proj1, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.Equal(t, usageConfig.Storage.Free, *proj1.StorageLimit)
		require.Equal(t, usageConfig.Bandwidth.Free, *proj1.BandwidthLimit)
		require.Equal(t, usageConfig.Segment.Free, *proj1.SegmentLimit)

		// user should be in free tier
		user, err := service.GetUser(ctx, proj1.OwnerID)
		require.NoError(t, err)
		require.False(t, user.PaidTier)

		authCtx, err := sat.AuthenticatedContext(ctx, user.ID)
		require.NoError(t, err)

		// add a credit card to the user
		err = service.Payments().AddCreditCard(authCtx, "test-cc-token")
		require.NoError(t, err)

		// expect user to be in paid tier
		user, err = service.GetUser(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, user.PaidTier)
		require.Equal(t, usageConfig.Project.Paid, user.ProjectLimit)

		// update auth ctx
		authCtx, err = sat.AuthenticatedContext(ctx, user.ID)
		require.NoError(t, err)

		// expect project to be migrated to paid tier usage limits
		proj1, err = service.GetProject(authCtx, proj1.ID)
		require.NoError(t, err)
		require.Equal(t, usageConfig.Storage.Paid, *proj1.StorageLimit)
		require.Equal(t, usageConfig.Bandwidth.Paid, *proj1.BandwidthLimit)
		require.Equal(t, usageConfig.Segment.Paid, *proj1.SegmentLimit)

		// expect new project to be created with paid tier usage limits
		proj2, err := service.CreateProject(authCtx, console.ProjectInfo{Name: "Project 2"})
		require.NoError(t, err)
		require.Equal(t, usageConfig.Storage.Paid, *proj2.StorageLimit)
	})
}

func TestMFA(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "MFA Test User",
			Email:    "mfauser@mail.test",
		}, 1)
		require.NoError(t, err)

		getNewAuthorization := func() (context.Context, console.Authorization) {
			authCtx, err := sat.AuthenticatedContext(ctx, user.ID)
			require.NoError(t, err)
			auth, err := console.GetAuth(authCtx)
			require.NoError(t, err)
			return authCtx, auth
		}
		authCtx, auth := getNewAuthorization()

		var key string
		t.Run("TestResetMFASecretKey", func(t *testing.T) {
			key, err = service.ResetMFASecretKey(authCtx)
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			require.NotEmpty(t, auth.User.MFASecretKey)
		})

		t.Run("TestEnableUserMFABadPasscode", func(t *testing.T) {
			// Expect MFA-enabling attempt to be rejected when providing stale passcode.
			badCode, err := console.NewMFAPasscode(key, time.Time{}.Add(time.Hour))
			require.NoError(t, err)

			err = service.EnableUserMFA(authCtx, badCode, time.Time{})
			require.True(t, console.ErrValidation.Has(err))

			authCtx, auth = getNewAuthorization()
			_, err = service.ResetMFARecoveryCodes(authCtx)
			require.True(t, console.ErrUnauthorized.Has(err))

			authCtx, auth = getNewAuthorization()
			require.False(t, auth.User.MFAEnabled)
		})

		t.Run("TestEnableUserMFAGoodPasscode", func(t *testing.T) {
			// Expect MFA-enabling attempt to succeed when providing valid passcode.
			goodCode, err := console.NewMFAPasscode(key, time.Time{})
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			err = service.EnableUserMFA(authCtx, goodCode, time.Time{})
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			require.True(t, auth.User.MFAEnabled)
			require.Equal(t, auth.User.MFASecretKey, key)
		})

		t.Run("TestMFAGetToken", func(t *testing.T) {
			request := console.AuthUser{Email: user.Email, Password: user.FullName}

			// Expect no token due to lack of MFA passcode.
			token, err := service.Token(ctx, request)
			require.True(t, console.ErrMFAMissing.Has(err))
			require.Empty(t, token)

			// Expect no token due to bad MFA passcode.
			wrongCode, err := console.NewMFAPasscode(key, time.Now().Add(time.Hour))
			require.NoError(t, err)

			request.MFAPasscode = wrongCode
			token, err = service.Token(ctx, request)
			require.True(t, console.ErrMFAPasscode.Has(err))
			require.Empty(t, token)

			// Expect token when providing valid passcode.
			goodCode, err := console.NewMFAPasscode(key, time.Now())
			require.NoError(t, err)

			request.MFAPasscode = goodCode
			token, err = service.Token(ctx, request)
			require.NoError(t, err)
			require.NotEmpty(t, token)
		})

		t.Run("TestMFARecoveryCodes", func(t *testing.T) {
			_, err = service.ResetMFARecoveryCodes(authCtx)
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			require.Len(t, auth.User.MFARecoveryCodes, console.MFARecoveryCodeCount)

			for _, code := range auth.User.MFARecoveryCodes {
				// Ensure code is of the form XXXX-XXXX-XXXX where X is A-Z or 0-9.
				require.Regexp(t, "^([A-Z0-9]{4})((-[A-Z0-9]{4})){2}$", code)

				// Expect token when providing valid recovery code.
				request := console.AuthUser{Email: user.Email, Password: user.FullName, MFARecoveryCode: code}
				token, err := service.Token(ctx, request)
				require.NoError(t, err)
				require.NotEmpty(t, token)

				// Expect no token due to providing previously-used recovery code.
				token, err = service.Token(ctx, request)
				require.True(t, console.ErrMFARecoveryCode.Has(err))
				require.Empty(t, token)

				authCtx, auth = getNewAuthorization()
			}

			_, err = service.ResetMFARecoveryCodes(authCtx)
			require.NoError(t, err)
		})

		t.Run("TestDisableUserMFABadPasscode", func(t *testing.T) {
			// Expect MFA-disabling attempt to fail when providing valid passcode.
			badCode, err := console.NewMFAPasscode(key, time.Time{}.Add(time.Hour))
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			err = service.DisableUserMFA(authCtx, badCode, time.Time{}, "")
			require.True(t, console.ErrValidation.Has(err))

			authCtx, auth = getNewAuthorization()
			require.True(t, auth.User.MFAEnabled)
			require.NotEmpty(t, auth.User.MFASecretKey)
			require.NotEmpty(t, auth.User.MFARecoveryCodes)
		})

		t.Run("TestDisableUserMFAConflict", func(t *testing.T) {
			// Expect MFA-disabling attempt to fail when providing both recovery code and passcode.
			goodCode, err := console.NewMFAPasscode(key, time.Time{})
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			err = service.DisableUserMFA(authCtx, goodCode, time.Time{}, auth.User.MFARecoveryCodes[0])
			require.True(t, console.ErrMFAConflict.Has(err))

			authCtx, auth = getNewAuthorization()
			require.True(t, auth.User.MFAEnabled)
			require.NotEmpty(t, auth.User.MFASecretKey)
			require.NotEmpty(t, auth.User.MFARecoveryCodes)
		})

		t.Run("TestDisableUserMFAGoodPasscode", func(t *testing.T) {
			// Expect MFA-disabling attempt to succeed when providing valid passcode.
			goodCode, err := console.NewMFAPasscode(key, time.Time{})
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			err = service.DisableUserMFA(authCtx, goodCode, time.Time{}, "")
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			require.False(t, auth.User.MFAEnabled)
			require.Empty(t, auth.User.MFASecretKey)
			require.Empty(t, auth.User.MFARecoveryCodes)
		})

		t.Run("TestDisableUserMFAGoodRecoveryCode", func(t *testing.T) {
			// Expect MFA-disabling attempt to succeed when providing valid recovery code.
			// Enable MFA
			key, err = service.ResetMFASecretKey(authCtx)
			require.NoError(t, err)

			goodCode, err := console.NewMFAPasscode(key, time.Time{})
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			err = service.EnableUserMFA(authCtx, goodCode, time.Time{})
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			_, err = service.ResetMFARecoveryCodes(authCtx)
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			require.True(t, auth.User.MFAEnabled)
			require.NotEmpty(t, auth.User.MFASecretKey)
			require.NotEmpty(t, auth.User.MFARecoveryCodes)

			// Disable MFA
			err = service.DisableUserMFA(authCtx, "", time.Time{}, auth.User.MFARecoveryCodes[0])
			require.NoError(t, err)

			authCtx, auth = getNewAuthorization()
			require.False(t, auth.User.MFAEnabled)
			require.Empty(t, auth.User.MFASecretKey)
			require.Empty(t, auth.User.MFARecoveryCodes)
		})
	})
}

func TestResetPassword(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		newPass := user.FullName

		getNewResetToken := func() *console.ResetPasswordToken {
			token, err := sat.DB.Console().ResetPasswordTokens().Create(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, token)
			return token
		}
		token := getNewResetToken()

		// Expect error when providing bad token.
		err = service.ResetPassword(ctx, "badToken", newPass, "", "", token.CreatedAt)
		require.True(t, console.ErrRecoveryToken.Has(err))

		// Expect error when providing good but expired token.
		err = service.ResetPassword(ctx, token.Secret.String(), newPass, "", "", token.CreatedAt.Add(sat.Config.Console.TokenExpirationTime).Add(time.Second))
		require.True(t, console.ErrTokenExpiration.Has(err))

		// Expect error when providing good token with bad (too short) password.
		err = service.ResetPassword(ctx, token.Secret.String(), "bad", "", "", token.CreatedAt)
		require.True(t, console.ErrValidation.Has(err))

		// Expect success when providing good token and good password.
		err = service.ResetPassword(ctx, token.Secret.String(), newPass, "", "", token.CreatedAt)
		require.NoError(t, err)

		token = getNewResetToken()

		// Enable MFA.
		getNewAuthorization := func() (context.Context, console.Authorization) {
			authCtx, err := sat.AuthenticatedContext(ctx, user.ID)
			require.NoError(t, err)
			auth, err := console.GetAuth(authCtx)
			require.NoError(t, err)
			return authCtx, auth
		}
		authCtx, _ := getNewAuthorization()

		key, err := service.ResetMFASecretKey(authCtx)
		require.NoError(t, err)
		authCtx, auth := getNewAuthorization()

		passcode, err := console.NewMFAPasscode(key, token.CreatedAt)
		require.NoError(t, err)

		err = service.EnableUserMFA(authCtx, passcode, token.CreatedAt)
		require.NoError(t, err)

		// Expect error when providing bad passcode.
		badPasscode, err := console.NewMFAPasscode(key, token.CreatedAt.Add(time.Hour))
		require.NoError(t, err)
		err = service.ResetPassword(ctx, token.Secret.String(), newPass, badPasscode, "", token.CreatedAt)
		require.True(t, console.ErrMFAPasscode.Has(err))

		for _, recoveryCode := range auth.User.MFARecoveryCodes {
			// Expect success when providing bad passcode and good recovery code.
			err = service.ResetPassword(ctx, token.Secret.String(), newPass, badPasscode, recoveryCode, token.CreatedAt)
			require.NoError(t, err)
			token = getNewResetToken()

			// Expect error when providing bad passcode and already-used recovery code.
			err = service.ResetPassword(ctx, token.Secret.String(), newPass, badPasscode, recoveryCode, token.CreatedAt)
			require.True(t, console.ErrMFARecoveryCode.Has(err))
		}

		// Expect success when providing good passcode.
		err = service.ResetPassword(ctx, token.Secret.String(), newPass, passcode, "", token.CreatedAt)
		require.NoError(t, err)
	})
}

// TestActivateAccountToken ensures that the token returned after activating an account can be used to authorize user activity.
// i.e. a user does not need to acquire an authorization separate from the account activation step.
func TestActivateAccountToken(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		createUser := console.CreateUser{
			FullName:  "Alice",
			ShortName: "Alice",
			Email:     "alice@mail.test",
			Password:  "123a123",
		}

		regToken, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		rootUser, err := service.CreateUser(ctx, createUser, regToken.Secret)
		require.NoError(t, err)

		activationToken, err := service.GenerateActivationToken(ctx, rootUser.ID, rootUser.Email)
		require.NoError(t, err)

		authToken, err := service.ActivateAccount(ctx, activationToken)
		require.NoError(t, err)

		_, err = service.Authorize(consoleauth.WithAPIKey(ctx, []byte(authToken)))
		require.NoError(t, err)

	})
}
