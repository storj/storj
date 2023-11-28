// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestUserGet(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		project := planet.Uplinks[0].Projects[0]

		projLimit, err := sat.DB.Console().Users().GetProjectLimit(ctx, project.Owner.ID)
		require.NoError(t, err)

		link := "http://" + address.String() + "/api/users/" + project.Owner.Email
		expectedBody := `{` +
			fmt.Sprintf(`"user":{"id":"%s","fullName":"User uplink0_0","email":"%s","projectLimit":%d,"placement":%d},`, project.Owner.ID, project.Owner.Email, projLimit, storj.DefaultPlacement) +
			fmt.Sprintf(`"projects":[{"id":"%s","name":"uplink0_0","description":"","ownerId":"%s"}]}`, project.ID, project.Owner.ID)

		assertReq(ctx, t, link, http.MethodGet, "", http.StatusOK, expectedBody, planet.Satellites[0].Config.Console.AuthToken)

		link = "http://" + address.String() + "/api/users/" + "user-not-exist@not-exist.test"
		body := assertReq(ctx, t, link, http.MethodGet, "", http.StatusNotFound, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Contains(t, string(body), "does not exist")
	})
}

func TestUserAdd(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		email := "alice+2@mail.test"

		body := strings.NewReader(fmt.Sprintf(`{"email":"%s","fullName":"Alice Test","password":"123a123"}`, email))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address.String()+"/api/users", body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))

		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())

		var output console.User

		err = json.Unmarshal(responseBody, &output)
		require.NoError(t, err)

		user, err := planet.Satellites[0].DB.Console().Users().Get(ctx, output.ID)
		require.NoError(t, err)
		require.Equal(t, email, user.Email)
	})
}

func TestUserAdd_sameEmail(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		email := "alice+2@mail.test"

		body := strings.NewReader(fmt.Sprintf(`{"email":"%s","fullName":"Alice Test","password":"123a123"}`, email))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address.String()+"/api/users", body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))

		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())

		var output console.User

		err = json.Unmarshal(responseBody, &output)
		require.NoError(t, err)

		user, err := planet.Satellites[0].DB.Console().Users().Get(ctx, output.ID)
		require.NoError(t, err)
		require.Equal(t, email, user.Email)

		// Add same user again, this should fail
		body = strings.NewReader(fmt.Sprintf(`{"email":"%s","fullName":"Alice Test","password":"123a123"}`, email))
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address.String()+"/api/users", body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))
		require.NoError(t, response.Body.Close())
	})
}

func TestUserUpdate(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmail(ctx, planet.Uplinks[0].Projects[0].Owner.Email)
		require.NoError(t, err)

		t.Run("OK", func(t *testing.T) {
			// Update user data.
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s", user.Email)
			body := `{"email":"alice+2@mail.test", "shortName":"Newbie"}`
			responseBody := assertReq(ctx, t, link, http.MethodPut, body, http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Len(t, responseBody, 0)

			updatedUser, err := planet.Satellites[0].DB.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, "alice+2@mail.test", updatedUser.Email)
			require.Equal(t, user.FullName, updatedUser.FullName)
			require.NotEqual(t, "Newbie", user.ShortName)
			require.Equal(t, "Newbie", updatedUser.ShortName)
			require.Equal(t, user.ID, updatedUser.ID)
			require.Equal(t, user.Status, updatedUser.Status)
			require.Equal(t, user.ProjectLimit, updatedUser.ProjectLimit)

			// Update project limit.
			link = "http://" + address.String() + "/api/users/alice+2@mail.test"
			newLimit := 50
			body = fmt.Sprintf(`{"projectLimit":%d}`, newLimit)
			responseBody = assertReq(ctx, t, link, http.MethodPut, body, http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Len(t, responseBody, 0)

			updatedUserProjectLimit, err := planet.Satellites[0].DB.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, updatedUser.Email, updatedUserProjectLimit.Email)
			require.Equal(t, updatedUser.ID, updatedUserProjectLimit.ID)
			require.Equal(t, updatedUser.Status, updatedUserProjectLimit.Status)
			require.Equal(t, newLimit, updatedUserProjectLimit.ProjectLimit)

			// Update paid tier status and usage.
			link = "http://" + address.String() + "/api/users/alice+2@mail.test"
			newUsageLimit := int64(1000)
			body1 := fmt.Sprintf(`{"projectStorageLimit":%d, "projectBandwidthLimit":%d, "projectSegmentLimit":%d, "paidTierStr":"true"}`, newUsageLimit, newUsageLimit, newUsageLimit)
			responseBody = assertReq(ctx, t, link, http.MethodPut, body1, http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Len(t, responseBody, 0)

			updatedUserStatusAndUsageLimits, err := planet.Satellites[0].DB.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, updatedUser.Email, updatedUserStatusAndUsageLimits.Email)
			require.Equal(t, updatedUser.ID, updatedUserStatusAndUsageLimits.ID)
			require.Equal(t, updatedUser.Status, updatedUserStatusAndUsageLimits.Status)
			require.True(t, updatedUserStatusAndUsageLimits.PaidTier)
			require.Equal(t, newUsageLimit, updatedUserStatusAndUsageLimits.ProjectStorageLimit)
			require.Equal(t, newUsageLimit, updatedUserStatusAndUsageLimits.ProjectBandwidthLimit)
			require.Equal(t, newUsageLimit, updatedUserStatusAndUsageLimits.ProjectSegmentLimit)

			var updateLimitsTests = []struct {
				newStorageLimit   memory.Size
				newBandwidthLimit memory.Size
				newSegmentLimit   int64
				useSizeString     bool
			}{
				{
					15000,
					25000,
					35000,
					false,
				},
				{
					50 * memory.KB,
					75 * memory.KB,
					40000,
					true,
				},
			}

			for _, tt := range updateLimitsTests {
				// Update user limits and project limits (current and existing projects for a user).
				link = "http://" + address.String() + "/api/users/alice+2@mail.test/limits"
				jsonStr := `{"storage":"%d", "bandwidth":"%d", "segment":%d}`
				if tt.useSizeString {
					jsonStr = strings.Replace(jsonStr, "%d", "%v", 2)
				}
				body2 := fmt.Sprintf(jsonStr, tt.newStorageLimit, tt.newBandwidthLimit, tt.newSegmentLimit)
				responseBody = assertReq(ctx, t, link, http.MethodPut, body2, http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
				require.Len(t, responseBody, 0)

				// Get user limits returns new updated limits
				link2 := "http://" + address.String() + "/api/users/alice+2@mail.test/limits"
				expectedBody := `{` +
					fmt.Sprintf(`"storage":%d,"bandwidth":%d,"segment":%d}`, tt.newStorageLimit, tt.newBandwidthLimit, tt.newSegmentLimit)
				assertReq(ctx, t, link2, http.MethodGet, "", http.StatusOK, expectedBody, planet.Satellites[0].Config.Console.AuthToken)

				userUpdatedLimits, err := planet.Satellites[0].DB.Console().Users().Get(ctx, user.ID)
				require.NoError(t, err)
				require.Equal(t, tt.newStorageLimit.Int64(), userUpdatedLimits.ProjectStorageLimit)
				require.Equal(t, tt.newBandwidthLimit.Int64(), userUpdatedLimits.ProjectBandwidthLimit)
				require.Equal(t, tt.newSegmentLimit, userUpdatedLimits.ProjectSegmentLimit)

				projectUpdatedLimits, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
				require.NoError(t, err)
				require.Equal(t, tt.newStorageLimit, *projectUpdatedLimits.StorageLimit)
				require.Equal(t, tt.newBandwidthLimit, *projectUpdatedLimits.BandwidthLimit)
				require.Equal(t, tt.newSegmentLimit, *projectUpdatedLimits.SegmentLimit)
			}
		})

		t.Run("Not found", func(t *testing.T) {
			link := "http://" + address.String() + "/api/users/user-not-exists@not-exists.test"
			body := `{"email":"alice+2@mail.test", "shortName":"Newbie"}`
			responseBody := assertReq(ctx, t, link, http.MethodPut, body, http.StatusNotFound, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Contains(t, string(responseBody), "does not exist")
		})

		t.Run("Email already used", func(t *testing.T) {
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s", "alice+2@mail.test")
			body := `{"email":"alice+2@mail.test", "shortName":"Newbie"}`
			responseBody := assertReq(ctx, t, link, http.MethodPut, body, http.StatusConflict, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Contains(t, string(responseBody), "already exists")
		})
	})
}

func TestUpdateUsersUserAgent(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		db := planet.Satellites[0].DB
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		project := planet.Uplinks[0].Projects[0]
		newUserAgent := "awesome user agent value"

		t.Run("OK", func(t *testing.T) {
			body := strings.NewReader(fmt.Sprintf(`{"userAgent":"%s"}`, newUserAgent))
			req, err := http.NewRequestWithContext(ctx, http.MethodPatch, fmt.Sprintf("http://"+address.String()+"/api/users/%s/useragent", project.Owner.Email), body)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			newUserAgentBytes := []byte(newUserAgent)

			updatedUser, err := db.Console().Users().Get(ctx, project.Owner.ID)
			require.NoError(t, err)
			require.Equal(t, newUserAgentBytes, updatedUser.UserAgent)

			updatedProject, err := db.Console().Projects().Get(ctx, project.ID)
			require.NoError(t, err)
			require.Equal(t, newUserAgentBytes, updatedProject.UserAgent)
		})

		t.Run("Same UserAgent", func(t *testing.T) {
			err := db.Console().Users().UpdateUserAgent(ctx, project.Owner.ID, []byte(newUserAgent))
			require.NoError(t, err)

			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/useragent", project.Owner.Email)
			body := fmt.Sprintf(`{"userAgent":"%s"}`, newUserAgent)
			responseBody := assertReq(ctx, t, link, http.MethodPatch, body, http.StatusBadRequest, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Contains(t, string(responseBody), "new UserAgent is equal to existing users UserAgent")
		})
	})
}

func TestDisableMFA(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmail(ctx, planet.Uplinks[0].Projects[0].Owner.Email)
		require.NoError(t, err)

		// Enable MFA.
		user.MFAEnabled = true
		user.MFASecretKey = "randomtext"
		user.MFARecoveryCodes = []string{"0123456789"}

		secretKeyPtr := &user.MFASecretKey

		err = planet.Satellites[0].DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
			MFAEnabled:       &user.MFAEnabled,
			MFASecretKey:     &secretKeyPtr,
			MFARecoveryCodes: &user.MFARecoveryCodes,
		})
		require.NoError(t, err)

		// Ensure MFA is enabled.
		updatedUser, err := planet.Satellites[0].DB.Console().Users().Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, true, updatedUser.MFAEnabled)

		// Disabling users MFA should work.
		link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/mfa", user.Email)
		body := assertReq(ctx, t, link, http.MethodDelete, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		// Ensure MFA is disabled.
		updatedUser, err = planet.Satellites[0].DB.Console().Users().Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, false, updatedUser.MFAEnabled)
	})
}

func TestBillingFreezeUnfreezeUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		userPreFreeze, err := planet.Satellites[0].DB.Console().Users().Get(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.NotZero(t, userPreFreeze.ProjectStorageLimit)
		require.NotZero(t, userPreFreeze.ProjectBandwidthLimit)

		projectPreFreeze, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.NotZero(t, projectPreFreeze.BandwidthLimit)
		require.NotZero(t, projectPreFreeze.StorageLimit)

		// freeze can be run multiple times. Test that doing so does not affect Unfreeze result.
		link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/billing-freeze", userPreFreeze.Email)
		body := assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		link = fmt.Sprintf("http://"+address.String()+"/api/users/%s/billing-freeze", userPreFreeze.Email)
		body = assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		userPostFreeze, err := planet.Satellites[0].DB.Console().Users().Get(ctx, userPreFreeze.ID)
		require.NoError(t, err)
		require.Zero(t, userPostFreeze.ProjectStorageLimit)
		require.Zero(t, userPostFreeze.ProjectBandwidthLimit)

		projectPostFreeze, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.Zero(t, projectPostFreeze.BandwidthLimit.Int64())
		require.Zero(t, projectPostFreeze.StorageLimit.Int64())

		link = fmt.Sprintf("http://"+address.String()+"/api/users/%s/billing-freeze", userPreFreeze.Email)
		body = assertReq(ctx, t, link, http.MethodDelete, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		unfrozenUser, err := planet.Satellites[0].DB.Console().Users().Get(ctx, userPreFreeze.ID)
		require.NoError(t, err)
		require.Equal(t, userPreFreeze.ProjectStorageLimit, unfrozenUser.ProjectStorageLimit)
		require.Equal(t, userPreFreeze.ProjectBandwidthLimit, unfrozenUser.ProjectBandwidthLimit)

		unfrozenProject, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, projectPreFreeze.ID)
		require.NoError(t, err)
		require.Equal(t, projectPreFreeze.StorageLimit, unfrozenProject.StorageLimit)
		require.Equal(t, projectPreFreeze.BandwidthLimit, unfrozenProject.BandwidthLimit)

		body = assertReq(ctx, t, link, http.MethodDelete, "", http.StatusNotFound, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Contains(t, string(body), console.ErrNoFreezeStatus.Error())
	})
}

func TestViolationFreezeUnfreezeUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		userPreFreeze, err := planet.Satellites[0].DB.Console().Users().Get(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.Equal(t, console.Active, userPreFreeze.Status)
		require.NotZero(t, userPreFreeze.ProjectStorageLimit)
		require.NotZero(t, userPreFreeze.ProjectBandwidthLimit)

		projectPreFreeze, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.NotZero(t, projectPreFreeze.BandwidthLimit)
		require.NotZero(t, projectPreFreeze.StorageLimit)

		// freeze can be run multiple times. Test that doing so does not affect Unfreeze result.
		link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/violation-freeze", userPreFreeze.Email)
		body := assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		body = assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		userPostFreeze, err := planet.Satellites[0].DB.Console().Users().Get(ctx, userPreFreeze.ID)
		require.NoError(t, err)
		require.Equal(t, console.PendingDeletion, userPostFreeze.Status)
		require.Zero(t, userPostFreeze.ProjectStorageLimit)
		require.Zero(t, userPostFreeze.ProjectBandwidthLimit)

		projectPostFreeze, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.Zero(t, projectPostFreeze.BandwidthLimit.Int64())
		require.Zero(t, projectPostFreeze.StorageLimit.Int64())

		link = fmt.Sprintf("http://"+address.String()+"/api/users/%s/violation-freeze", userPreFreeze.Email)
		body = assertReq(ctx, t, link, http.MethodDelete, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		unfrozenUser, err := planet.Satellites[0].DB.Console().Users().Get(ctx, userPreFreeze.ID)
		require.NoError(t, err)
		require.Equal(t, console.Active, unfrozenUser.Status)
		require.Equal(t, userPreFreeze.ProjectStorageLimit, unfrozenUser.ProjectStorageLimit)
		require.Equal(t, userPreFreeze.ProjectBandwidthLimit, unfrozenUser.ProjectBandwidthLimit)

		unfrozenProject, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, projectPreFreeze.ID)
		require.NoError(t, err)
		require.Equal(t, projectPreFreeze.StorageLimit, unfrozenProject.StorageLimit)
		require.Equal(t, projectPreFreeze.BandwidthLimit, unfrozenProject.BandwidthLimit)

		body = assertReq(ctx, t, link, http.MethodDelete, "", http.StatusNotFound, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Contains(t, string(body), console.ErrNoFreezeStatus.Error())
	})
}

func TestLegalFreezeUnfreezeUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		userPreFreeze, err := planet.Satellites[0].DB.Console().Users().Get(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.Equal(t, console.Active, userPreFreeze.Status)
		require.NotZero(t, userPreFreeze.ProjectStorageLimit)
		require.NotZero(t, userPreFreeze.ProjectBandwidthLimit)

		projectPreFreeze, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.NotZero(t, projectPreFreeze.BandwidthLimit)
		require.NotZero(t, projectPreFreeze.StorageLimit)

		// freeze can be run multiple times. Test that doing so does not affect Unfreeze result.
		link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/legal-freeze", userPreFreeze.Email)
		body := assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		body = assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		userPostFreeze, err := planet.Satellites[0].DB.Console().Users().Get(ctx, userPreFreeze.ID)
		require.NoError(t, err)
		require.Equal(t, console.LegalHold, userPostFreeze.Status)
		require.Zero(t, userPostFreeze.ProjectStorageLimit)
		require.Zero(t, userPostFreeze.ProjectBandwidthLimit)

		projectPostFreeze, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.Zero(t, projectPostFreeze.BandwidthLimit.Int64())
		require.Zero(t, projectPostFreeze.StorageLimit.Int64())

		link = fmt.Sprintf("http://"+address.String()+"/api/users/%s/legal-freeze", userPreFreeze.Email)
		body = assertReq(ctx, t, link, http.MethodDelete, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		unfrozenUser, err := planet.Satellites[0].DB.Console().Users().Get(ctx, userPreFreeze.ID)
		require.NoError(t, err)
		require.Equal(t, console.Active, unfrozenUser.Status)
		require.Equal(t, userPreFreeze.ProjectStorageLimit, unfrozenUser.ProjectStorageLimit)
		require.Equal(t, userPreFreeze.ProjectBandwidthLimit, unfrozenUser.ProjectBandwidthLimit)

		unfrozenProject, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, projectPreFreeze.ID)
		require.NoError(t, err)
		require.Equal(t, projectPreFreeze.StorageLimit, unfrozenProject.StorageLimit)
		require.Equal(t, projectPreFreeze.BandwidthLimit, unfrozenProject.BandwidthLimit)

		body = assertReq(ctx, t, link, http.MethodDelete, "", http.StatusNotFound, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Contains(t, string(body), console.ErrNoFreezeStatus.Error())
	})
}

func TestBillingWarnUnwarnUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		user, err := planet.Satellites[0].DB.Console().Users().Get(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)

		err = planet.Satellites[0].Admin.FreezeAccounts.Service.BillingWarnUser(ctx, user.ID)
		require.NoError(t, err)

		freezes, err := planet.Satellites[0].DB.Console().AccountFreezeEvents().GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.BillingWarning)

		link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/billing-warning", user.Email)
		body := assertReq(ctx, t, link, http.MethodDelete, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		freezes, err = planet.Satellites[0].DB.Console().AccountFreezeEvents().GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.Nil(t, freezes.BillingWarning)

		body = assertReq(ctx, t, link, http.MethodDelete, "", http.StatusNotFound, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Contains(t, string(body), console.ErrNoFreezeStatus.Error())
	})
}

func TestUserDelete(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmail(ctx, planet.Uplinks[0].Projects[0].Owner.Email)
		require.NoError(t, err)

		// Deleting the user should fail, as project exists.
		link := fmt.Sprintf("http://"+address.String()+"/api/users/%s", user.Email)
		body := assertReq(ctx, t, link, http.MethodDelete, "", http.StatusConflict, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Greater(t, len(body), 0)

		err = planet.Satellites[0].DB.Console().Projects().Delete(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)

		// Deleting the user should pass, as no project exists for given user.
		body = assertReq(ctx, t, link, http.MethodDelete, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		// Deleting non-existing user returns Not Found.
		body = assertReq(ctx, t, link, http.MethodDelete, "", http.StatusNotFound, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Contains(t, string(body), "does not exist")
	})
}

func TestSetUsersGeofence(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		db := planet.Satellites[0].DB
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		project := planet.Uplinks[0].Projects[0]
		newPlacement := storj.EU
		newPlacementStr := "EU"
		link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/geofence", project.Owner.Email)

		t.Run("OK", func(t *testing.T) {
			body := fmt.Sprintf(`{"region":"%s"}`, newPlacementStr)
			assertReq(ctx, t, link, http.MethodPatch, body, http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)

			updatedUser, err := db.Console().Users().Get(ctx, project.Owner.ID)
			require.NoError(t, err)
			require.Equal(t, newPlacement, updatedUser.DefaultPlacement)

			// DELETE
			assertReq(ctx, t, link, http.MethodDelete, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
			updatedUser, err = db.Console().Users().Get(ctx, project.Owner.ID)
			require.NoError(t, err)
			require.Equal(t, storj.DefaultPlacement, updatedUser.DefaultPlacement)
		})

		t.Run("Same Placement", func(t *testing.T) {
			err := db.Console().Users().Update(ctx, project.Owner.ID, console.UpdateUserRequest{
				Email:            &project.Owner.Email,
				DefaultPlacement: newPlacement,
			})
			require.NoError(t, err)

			body := fmt.Sprintf(`{"region":"%s"}`, newPlacementStr)
			responseBody := assertReq(ctx, t, link, http.MethodPatch, body, http.StatusBadRequest, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Contains(t, string(responseBody), "new placement is equal to user's current placement")
		})
	})
}
