// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
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

		user, err := sat.DB.Console().Users().Get(ctx, project.Owner.ID)
		require.NoError(t, err)

		projLimit, err := sat.DB.Console().Users().GetProjectLimit(ctx, project.Owner.ID)
		require.NoError(t, err)

		link := "http://" + address.String() + "/api/users/" + project.Owner.Email
		expectedBody := `{` +
			fmt.Sprintf(
				`"user":{"id":"%s","fullName":"User uplink0_0","email":"%s","projectLimit":%d,"placement":%d,"paidTier":%t},`,
				project.Owner.ID,
				project.Owner.Email,
				projLimit,
				storj.DefaultPlacement,
				user.IsPaid(),
			) +
			fmt.Sprintf(
				`"projects":[{"id":"%s","publicId":"%s","name":"uplink0_0","description":"","ownerId":"%s"}]}`,
				project.ID,
				project.PublicID,
				project.Owner.ID,
			)

		assertReq(ctx, t, link, http.MethodGet, "", http.StatusOK, expectedBody, planet.Satellites[0].Config.Console.AuthToken)

		link = "http://" + address.String() + "/api/users/" + "user-not-exist@not-exist.test"
		body := assertReq(
			ctx,
			t,
			link,
			http.MethodGet,
			"",
			http.StatusNotFound,
			"",
			planet.Satellites[0].Config.Console.AuthToken,
		)
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

		body := strings.NewReader(fmt.Sprintf(`{"email":"%s","fullName":"Alice Test","password":"password"}`, email))
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

		body := strings.NewReader(fmt.Sprintf(`{"email":"%s","fullName":"Alice Test","password":"password"}`, email))
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
		body = strings.NewReader(fmt.Sprintf(`{"email":"%s","fullName":"Alice Test","password":"password"}`, email))
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
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].Projects[0].Owner.Email, nil)
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

			now := time.Now()
			planet.Satellites[0].Admin.Admin.Server.SetNow(func() time.Time {
				return now
			})

			// Update user kind and usage.
			link = "http://" + address.String() + "/api/users/alice+2@mail.test"
			newUsageLimit := int64(1000)
			body1 := fmt.Sprintf(`{"projectStorageLimit":%d, "projectBandwidthLimit":%d, "projectSegmentLimit":%d, "userKind":%d}`, newUsageLimit, newUsageLimit, newUsageLimit, console.PaidUser)
			responseBody = assertReq(ctx, t, link, http.MethodPut, body1, http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Len(t, responseBody, 0)

			updatedUserStatusAndUsageLimits, err := planet.Satellites[0].DB.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, updatedUser.Email, updatedUserStatusAndUsageLimits.Email)
			require.Equal(t, updatedUser.ID, updatedUserStatusAndUsageLimits.ID)
			require.Equal(t, updatedUser.Status, updatedUserStatusAndUsageLimits.Status)
			require.Equal(t, console.PaidUser, updatedUserStatusAndUsageLimits.Kind)
			require.Equal(t, newUsageLimit, updatedUserStatusAndUsageLimits.ProjectStorageLimit)
			require.Equal(t, newUsageLimit, updatedUserStatusAndUsageLimits.ProjectBandwidthLimit)
			require.Equal(t, newUsageLimit, updatedUserStatusAndUsageLimits.ProjectSegmentLimit)
			require.WithinDuration(t, now, *updatedUserStatusAndUsageLimits.UpgradeTime, time.Minute)

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

func TestUserStatusUpdate(t *testing.T) {
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
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].Projects[0].Owner.Email, nil)
		require.NoError(t, err)

		t.Run("OK", func(t *testing.T) {
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/status/%d", user.Email, console.PendingDeletion)
			responseBody := assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Len(t, responseBody, 0)

			updatedUser, err := planet.Satellites[0].DB.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.NotEqual(t, user.Status, updatedUser.Status)
			require.Equal(t, updatedUser.Status, console.PendingDeletion)
		})

		t.Run("invalid value", func(t *testing.T) {
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/status/not-int", user.Email)
			responseBody := assertReq(ctx, t, link, http.MethodPut, "", http.StatusBadRequest, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Contains(t, string(responseBody), "invalid user status. int required")
		})

		t.Run("invalid status", func(t *testing.T) {
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/status/%d", user.Email, console.UserStatus(100))
			responseBody := assertReq(ctx, t, link, http.MethodPut, "", http.StatusBadRequest, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Contains(t, string(responseBody), "invalid user status. There isn't any status with this value")
		})
	})
}

func TestUserKindUpdate(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Console.UsageLimits.Storage = console.StorageLimitConfig{
					Free: 25 * memory.GB,
					Paid: 50 * memory.GB,
					Nfr:  30 * memory.GB,
				}
				config.Console.UsageLimits.Bandwidth = console.BandwidthLimitConfig{
					Free: 25 * memory.GB,
					Paid: 50 * memory.GB,
					Nfr:  30 * memory.GB,
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		usageLimitsConfig := sat.Config.Console.UsageLimits

		user, err := sat.DB.Console().Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].Projects[0].Owner.Email, nil)
		require.NoError(t, err)
		require.Equal(t, console.FreeUser, user.Kind)

		projectID := planet.Uplinks[0].Projects[0].ID

		checkProjectLimits := func(kind console.UserKind) {
			var (
				storage   memory.Size
				bandwidth memory.Size
				segment   int64
			)
			switch kind {
			case console.PaidUser:
				storage, bandwidth, segment = usageLimitsConfig.Storage.Paid, usageLimitsConfig.Bandwidth.Paid, usageLimitsConfig.Segment.Paid
			case console.NFRUser:
				storage, bandwidth, segment = usageLimitsConfig.Storage.Nfr, usageLimitsConfig.Bandwidth.Nfr, usageLimitsConfig.Segment.Nfr
			case console.FreeUser:
				storage, bandwidth, segment = usageLimitsConfig.Storage.Free, usageLimitsConfig.Bandwidth.Free, usageLimitsConfig.Segment.Free
			}
			project, err := sat.DB.Console().Projects().Get(ctx, projectID)
			require.NoError(t, err)
			require.Equal(t, storage, *project.StorageLimit)
			require.Equal(t, bandwidth, *project.BandwidthLimit)
			require.Equal(t, segment, *project.SegmentLimit)
		}

		checkProjectLimits(console.FreeUser)

		t.Run("OK", func(t *testing.T) {
			updateKind := func(kind console.UserKind) {
				link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/kind/%d", user.Email, kind)
				_ = assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", sat.Config.Console.AuthToken)

				var (
					storage, bandwidth memory.Size
					segment            int64
					projectLimit       int
				)
				switch kind {
				case console.PaidUser:
					storage, bandwidth, segment, projectLimit = usageLimitsConfig.Storage.Paid, usageLimitsConfig.Bandwidth.Paid,
						usageLimitsConfig.Segment.Paid, usageLimitsConfig.Project.Paid
				case console.NFRUser:
					storage, bandwidth, segment, projectLimit = usageLimitsConfig.Storage.Nfr, usageLimitsConfig.Bandwidth.Nfr,
						usageLimitsConfig.Segment.Nfr, usageLimitsConfig.Project.Nfr
				case console.FreeUser:
					storage, bandwidth, segment, projectLimit = usageLimitsConfig.Storage.Free, usageLimitsConfig.Bandwidth.Free,
						usageLimitsConfig.Segment.Free, usageLimitsConfig.Project.Free
				}

				updatedUser, err := sat.DB.Console().Users().Get(ctx, user.ID)
				require.NoError(t, err)
				require.Equal(t, updatedUser.Kind, kind)
				require.Equal(t, projectLimit, updatedUser.ProjectLimit)
				require.Equal(t, storage.Int64(), updatedUser.ProjectStorageLimit)
				require.Equal(t, bandwidth.Int64(), updatedUser.ProjectBandwidthLimit)
				require.Equal(t, segment, updatedUser.ProjectSegmentLimit)
				if kind == console.PaidUser {
					require.NotNil(t, updatedUser.UpgradeTime)
				}

				checkProjectLimits(kind)
			}

			for _, kind := range []console.UserKind{console.PaidUser, console.NFRUser, console.FreeUser} {
				updateKind(kind)
			}

			freezeUser := func() {
				link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/trial-expiration-freeze", user.Email)
				_ = assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", sat.Config.Console.AuthToken)

				_, err = sat.DB.Console().AccountFreezeEvents().Get(ctx, user.ID, console.TrialExpirationFreeze)
				require.NoError(t, err)
			}

			freezeUser()

			// updating to console.PaidUser should remove the trial freeze event.
			updateKind(console.PaidUser)

			_, err = sat.DB.Console().AccountFreezeEvents().Get(ctx, user.ID, console.TrialExpirationFreeze)
			require.ErrorIs(t, err, sql.ErrNoRows)

			// reset to console.FreeUser.
			updateKind(console.FreeUser)

			// freeze again.
			freezeUser()

			// updating to console.NFRUser should remove the trial freeze event.
			updateKind(console.NFRUser)

			_, err = sat.DB.Console().AccountFreezeEvents().Get(ctx, user.ID, console.TrialExpirationFreeze)
			require.ErrorIs(t, err, sql.ErrNoRows)

			// set MemberUser kind.
			memberUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "member User",
				Email:    "member@example.com",
				Password: "password",
			}, 1)
			require.NoError(t, err)
			require.Equal(t, console.FreeUser, memberUser.Kind)

			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/kind/%d", memberUser.Email, console.MemberUser)
			_ = assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", sat.Config.Console.AuthToken)

			memberUser, err = sat.DB.Console().Users().Get(ctx, memberUser.ID)
			require.NoError(t, err)
			require.Equal(t, console.MemberUser, memberUser.Kind)
			require.Nil(t, memberUser.TrialExpiration)
		})

		t.Run("invalid value", func(t *testing.T) {
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/kind/not-int", user.Email)
			responseBody := assertReq(ctx, t, link, http.MethodPut, "", http.StatusBadRequest, "", sat.Config.Console.AuthToken)
			require.Contains(t, string(responseBody), "invalid user kind. int required")
		})

		t.Run("invalid kind", func(t *testing.T) {
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/kind/%d", user.Email, console.UserKind(100))
			responseBody := assertReq(ctx, t, link, http.MethodPut, "", http.StatusBadRequest, "", sat.Config.Console.AuthToken)
			require.Contains(t, string(responseBody), "invalid user kind 100")
		})

		t.Run("failed to set Member kind for user with own projects", func(t *testing.T) {
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/kind/%d", user.Email, console.MemberUser)
			responseBody := assertReq(ctx, t, link, http.MethodPut, "", http.StatusForbidden, "", sat.Config.Console.AuthToken)
			require.Contains(t, string(responseBody), "cannot change user kind to MemberUser when user has own projects")
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

func TestUpdateUsersExternalID(t *testing.T) {
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
		newExternalID := "some-id"

		t.Run("OK", func(t *testing.T) {
			url := fmt.Sprintf("http://"+address.String()+"/api/users/%s/external-id", project.Owner.Email)
			body := strings.NewReader(fmt.Sprintf(`{"externalID":"%s"}`, newExternalID))
			req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, body)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			updatedUser, err := db.Console().Users().Get(ctx, project.Owner.ID)
			require.NoError(t, err)
			require.NotNil(t, updatedUser.ExternalID)
			require.Equal(t, newExternalID, *updatedUser.ExternalID)

			// update to nil.
			body = strings.NewReader(`{"externalID":null}`)
			req, err = http.NewRequestWithContext(ctx, http.MethodPatch, url, body)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			updatedUser, err = db.Console().Users().Get(ctx, project.Owner.ID)
			require.NoError(t, err)
			require.Nil(t, updatedUser.ExternalID)
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
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].Projects[0].Owner.Email, nil)
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

func TestUpdateTrialExpiration(t *testing.T) {
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

		newExpirationDate := time.Now().UTC().Add(5 * 24 * time.Hour)

		body := strings.NewReader(fmt.Sprintf(`{"trialExpiration":"%s"}`, newExpirationDate.Format(time.RFC3339Nano)))
		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, fmt.Sprintf("http://"+address.String()+"/api/users/%s/trial-expiration", project.Owner.Email), body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.NoError(t, response.Body.Close())

		updatedUser, err := db.Console().Users().Get(ctx, project.Owner.ID)
		require.NoError(t, err)
		require.WithinDuration(t, newExpirationDate, *updatedUser.TrialExpiration, time.Minute)

		body = strings.NewReader(`{"trialExpiration":null}`)
		req, err = http.NewRequestWithContext(ctx, http.MethodPatch, fmt.Sprintf("http://"+address.String()+"/api/users/%s/trial-expiration", project.Owner.Email), body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.NoError(t, response.Body.Close())

		updatedUser, err = db.Console().Users().Get(ctx, project.Owner.ID)
		require.NoError(t, err)
		require.Nil(t, updatedUser.TrialExpiration)
	})
}

func TestDisableBotRestriction(t *testing.T) {
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
		consoleDB := sat.DB.Console()
		address := sat.Admin.Admin.Listener.Addr()
		user, err := consoleDB.Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].Projects[0].Owner.Email, nil)
		require.NoError(t, err)

		// Error on try set active status for a user with non-PendingBotVerification status.
		link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/activate-account/disable-bot-restriction", user.Email)
		expectedBody := fmt.Sprintf("{\"error\":\"user with email \\\"%s\\\" must have PendingBotVerification status to disable bot restriction\",\"detail\":\"\"}", user.Email)
		body := assertReq(ctx, t, link, http.MethodPatch, "", http.StatusBadRequest, expectedBody, sat.Config.Console.AuthToken)
		require.NotZero(t, len(body))

		// Update user status to pending bot verification and set zero limits.
		zeroLimit := int64(0)
		botStatus := console.PendingBotVerification
		err = consoleDB.Users().Update(ctx, user.ID, console.UpdateUserRequest{
			Status:                &botStatus,
			ProjectStorageLimit:   &zeroLimit,
			ProjectBandwidthLimit: &zeroLimit,
			ProjectSegmentLimit:   &zeroLimit,
		})
		require.NoError(t, err)

		// Error on try unfreeze user without BotFreeze event set.
		expectedBody = "{\"error\":\"failed to unfreeze bot user\",\"detail\":\"account freeze service: this freeze event does not exist for this user\"}"
		body = assertReq(ctx, t, link, http.MethodPatch, "", http.StatusConflict, expectedBody, sat.Config.Console.AuthToken)
		require.NotZero(t, len(body))

		// Prepare BotFreeze event.
		initUserLimits := console.UsageLimits{
			Storage:   user.ProjectStorageLimit,
			Bandwidth: user.ProjectBandwidthLimit,
			Segment:   user.ProjectSegmentLimit,
		}
		limits := &console.AccountFreezeEventLimits{
			User:     initUserLimits,
			Projects: make(map[uuid.UUID]console.UsageLimits),
		}
		botFreeze := &console.AccountFreezeEvent{
			UserID: user.ID,
			Type:   console.BotFreeze,
			Limits: limits,
		}

		// Insert BotFreeze event.
		_, err = consoleDB.AccountFreezeEvents().Upsert(ctx, botFreeze)
		require.NoError(t, err)

		// Restore user status and limits.
		body = assertReq(ctx, t, link, http.MethodPatch, "", http.StatusOK, "", sat.Config.Console.AuthToken)
		require.Len(t, body, 0)

		// Ensure status is updated.
		updatedUser, err := consoleDB.Users().Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, console.Active, updatedUser.Status)
		require.Equal(t, user.ProjectStorageLimit, updatedUser.ProjectStorageLimit)
		require.Equal(t, user.ProjectBandwidthLimit, updatedUser.ProjectBandwidthLimit)
		require.Equal(t, user.ProjectSegmentLimit, updatedUser.ProjectSegmentLimit)

		// BotFreeze event no longer exists.
		event, err := consoleDB.AccountFreezeEvents().Get(ctx, user.ID, console.BotFreeze)
		require.Error(t, err)
		require.True(t, errs.Is(err, sql.ErrNoRows))
		require.Nil(t, event)
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

func TestTrialExpirationFreezeUnfreezeUser(t *testing.T) {
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

		burstRateLimit := 1000
		err = planet.Satellites[0].DB.Console().Projects().UpdateBurstLimit(ctx, planet.Uplinks[0].Projects[0].ID, &burstRateLimit)
		require.NoError(t, err)
		err = planet.Satellites[0].DB.Console().Projects().UpdateRateLimit(ctx, planet.Uplinks[0].Projects[0].ID, &burstRateLimit)
		require.NoError(t, err)

		projectPreFreeze, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.NotZero(t, projectPreFreeze.BandwidthLimit)
		require.NotZero(t, projectPreFreeze.StorageLimit)
		require.NotZero(t, projectPreFreeze.BurstLimit)
		require.NotZero(t, projectPreFreeze.RateLimit)

		// freeze can be run multiple times. Test that doing so does not affect Unfreeze result.
		link := fmt.Sprintf("http://"+address.String()+"/api/users/%s/trial-expiration-freeze", userPreFreeze.Email)
		body := assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		body = assertReq(ctx, t, link, http.MethodPut, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)
		require.Len(t, body, 0)

		userPostFreeze, err := planet.Satellites[0].DB.Console().Users().Get(ctx, userPreFreeze.ID)
		require.NoError(t, err)
		require.Equal(t, console.Active, userPostFreeze.Status)
		require.Zero(t, userPostFreeze.ProjectStorageLimit)
		require.Zero(t, userPostFreeze.ProjectBandwidthLimit)

		projectPostFreeze, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.Zero(t, projectPostFreeze.BandwidthLimit.Int64())
		require.Zero(t, projectPostFreeze.StorageLimit.Int64())
		require.Zero(t, *projectPostFreeze.RateLimit)
		require.Zero(t, *projectPostFreeze.BurstLimit)

		link = fmt.Sprintf("http://"+address.String()+"/api/users/%s/trial-expiration-freeze", userPreFreeze.Email)
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
		require.Equal(t, projectPreFreeze.RateLimit, unfrozenProject.RateLimit)
		require.Equal(t, projectPreFreeze.BurstLimit, unfrozenProject.BurstLimit)

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
	t.Run("no member of foreign projects", func(t *testing.T) {
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
			user, err := planet.Satellites[0].DB.Console().Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].Projects[0].Owner.Email, nil)
			require.NoError(t, err)

			// Deleting the user should fail, as project exists.
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s", user.Email)
			body := assertReq(
				ctx,
				t,
				link,
				http.MethodDelete,
				"",
				http.StatusConflict,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Greater(t, len(body), 0)

			err = planet.Satellites[0].DB.Console().Projects().Delete(ctx, planet.Uplinks[0].Projects[0].ID)
			require.NoError(t, err)

			// Deleting the user should pass, as no project exists for given user.
			body = assertReq(
				ctx,
				t,
				link,
				http.MethodDelete,
				"",
				http.StatusOK,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Len(t, body, 0)

			// Deleting non-existing user returns Not Found.
			body = assertReq(
				ctx,
				t,
				link,
				http.MethodDelete,
				"",
				http.StatusNotFound,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Contains(t, string(body), "does not exist")
		})
	})

	t.Run("member of foreign projects", func(t *testing.T) {
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
			dbconsole := planet.Satellites[0].DB.Console()
			address := planet.Satellites[0].Admin.Admin.Listener.Addr()
			user, err := dbconsole.Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].Projects[0].Owner.Email, nil)
			require.NoError(t, err)

			userSharing, err := dbconsole.Users().Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				FullName:     "Sharing",
				Email:        testrand.UUID().String() + "@storj.test",
				PasswordHash: testrand.UUID().Bytes(),
			})
			require.NoError(t, err)

			sharedProject, err := dbconsole.Projects().Insert(ctx, &console.Project{
				Name:    "sharing",
				OwnerID: userSharing.ID,
			})
			require.NoError(t, err)

			members, err := dbconsole.ProjectMembers().
				GetPagedWithInvitationsByProjectID(ctx, sharedProject.ID, console.ProjectMembersCursor{Limit: 2, Page: 1})
			require.NoError(t, err)
			require.EqualValues(t, 0, members.TotalCount)

			_, err = dbconsole.ProjectMembers().Insert(ctx, user.ID, sharedProject.ID, console.RoleAdmin)
			require.NoError(t, err)

			members, err = dbconsole.ProjectMembers().
				GetPagedWithInvitationsByProjectID(ctx, sharedProject.ID, console.ProjectMembersCursor{Limit: 2, Page: 1})
			require.NoError(t, err)
			require.EqualValues(t, 1, members.TotalCount)

			err = planet.Satellites[0].DB.Console().Projects().Delete(ctx, planet.Uplinks[0].Projects[0].ID)
			require.NoError(t, err)

			// Deleting the user should pass, as no project exists for given user.
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s", user.Email)
			body := assertReq(
				ctx,
				t,
				link,
				http.MethodDelete,
				"",
				http.StatusOK,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Len(t, body, 0)

			// The user marked as deleted should remain as member of the project because we keep this information for
			// historical data analysis.
			members, err = dbconsole.ProjectMembers().
				GetPagedWithInvitationsByProjectID(ctx, sharedProject.ID, console.ProjectMembersCursor{Limit: 2, Page: 1})
			require.NoError(t, err)
			require.EqualValues(t, 1, members.TotalCount)
		})
	})
	t.Run("has active project", func(t *testing.T) {
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
			dbconsole := planet.Satellites[0].DB.Console()
			address := planet.Satellites[0].Admin.Admin.Listener.Addr()
			user, err := dbconsole.Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].Projects[0].Owner.Email, nil)
			require.NoError(t, err)

			p, err := dbconsole.Projects().GetOwnActive(ctx, user.ID)
			require.NoError(t, err)
			require.Len(t, p, 1)

			// Deleting the user should fail, as active project exists for given user.
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s", user.Email)
			body := assertReq(
				ctx,
				t,
				link,
				http.MethodDelete,
				"",
				http.StatusConflict,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Greater(t, len(body), 0)

			user, err = dbconsole.Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.NotContains(t, user.Email, "deactivated")
		})
	})
	t.Run("has no active projects", func(t *testing.T) {
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
			dbconsole := planet.Satellites[0].DB.Console()
			address := planet.Satellites[0].Admin.Admin.Listener.Addr()
			user, err := dbconsole.Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].Projects[0].Owner.Email, nil)
			require.NoError(t, err)

			p, err := dbconsole.Projects().GetOwnActive(ctx, user.ID)
			require.NoError(t, err)
			require.Len(t, p, 1)

			require.NoError(t, dbconsole.Projects().UpdateStatus(ctx, p[0].ID, console.ProjectDisabled))

			// Deleting the user should pass, as no active project exists for given user.
			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s", user.Email)
			body := assertReq(
				ctx,
				t,
				link,
				http.MethodDelete,
				"",
				http.StatusOK,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Len(t, body, 0)

			user, err = dbconsole.Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.Contains(t, user.Email, "deactivated")
		})
	})

	t.Run("external ID is set to nil", func(t *testing.T) {
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
			dbconsole := planet.Satellites[0].DB.Console()
			address := planet.Satellites[0].Admin.Admin.Listener.Addr()
			user, err := dbconsole.Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].Projects[0].Owner.Email, nil)
			require.NoError(t, err)

			require.NoError(t, dbconsole.Projects().UpdateStatus(ctx, planet.Uplinks[0].Projects[0].ID, console.ProjectDisabled))

			externalID := "external-id"
			externalIDPtr := &externalID
			err = dbconsole.Users().Update(ctx, user.ID, console.UpdateUserRequest{ExternalID: &externalIDPtr})
			require.NoError(t, err)

			user, err = dbconsole.Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, user.ExternalID)
			require.Equal(t, externalID, *user.ExternalID)

			link := fmt.Sprintf("http://"+address.String()+"/api/users/%s", user.Email)
			body := assertReq(
				ctx,
				t,
				link,
				http.MethodDelete,
				"",
				http.StatusOK,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Len(t, body, 0)

			user, err = dbconsole.Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.Contains(t, user.Email, "deactivated")
			require.Nil(t, user.ExternalID)
		})
	})
}
