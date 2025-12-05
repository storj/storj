// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	backoffice "storj.io/storj/satellite/admin/back-office"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestGetUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.LiveAccounting.AsOfSystemInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()

		_, apiErr := service.GetUserByEmail(ctx, "test@test.test")
		require.Equal(t, http.StatusNotFound, apiErr.Status)
		require.Error(t, apiErr.Err)
		_, apiErr = service.GetUser(ctx, testrand.UUID())
		require.Equal(t, http.StatusNotFound, apiErr.Status)
		require.Error(t, apiErr.Err)

		consoleUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName:  "Test User",
			Email:     "test@test.test",
			UserAgent: []byte("agent"),
		}, 1)
		require.NoError(t, err)
		consoleUser.Status = console.Inactive
		require.NoError(t, consoleDB.Users().Update(ctx, consoleUser.ID, console.UpdateUserRequest{Status: &consoleUser.Status}))

		consoleUser.Kind = console.PaidUser
		require.NoError(
			t,
			sat.DB.Console().Users().Update(ctx, consoleUser.ID, console.UpdateUserRequest{Kind: &consoleUser.Kind}),
		)

		// User is deactivated, so it cannot be retrieved by e-mail.
		_, apiErr = service.GetUserByEmail(ctx, consoleUser.Email)
		require.Equal(t, http.StatusNotFound, apiErr.Status)
		require.Error(t, apiErr.Err)
		// can be retrieved by ID though.
		_, apiErr = service.GetUser(ctx, consoleUser.ID)
		require.NoError(t, apiErr.Err)

		consoleUser.Status = console.Active
		require.NoError(
			t,
			consoleDB.Users().Update(ctx, consoleUser.ID, console.UpdateUserRequest{Status: &consoleUser.Status}),
		)

		testUserFields := func(expected *console.User, actual *backoffice.UserAccount) {
			require.Equal(t, expected.ID, actual.User.ID)
			require.Equal(t, expected.FullName, actual.User.FullName)
			require.Equal(t, expected.Email, actual.User.Email)
			require.Equal(t, expected.Kind.Info(), actual.Kind)
			require.Equal(t, expected.Status.Info(), actual.Status)
			require.Equal(t, string(expected.UserAgent), actual.UserAgent)
			require.Equal(t, expected.DefaultPlacement, actual.DefaultPlacement)
		}

		user, apiErr := service.GetUserByEmail(ctx, consoleUser.Email)
		require.NoError(t, apiErr.Err)
		require.NotNil(t, user)
		testUserFields(consoleUser, user)
		require.Empty(t, user.Projects)

		user, apiErr = service.GetUser(ctx, consoleUser.ID)
		require.NoError(t, apiErr.Err)
		require.NotNil(t, user)
		testUserFields(consoleUser, user)
		require.Empty(t, user.Projects)

		type expectedTotal struct {
			storage   int64
			segments  int64
			bandwidth int64
			objects   int64
		}

		var projects []*console.Project
		var expectedTotals []expectedTotal

		for projNum := 1; projNum <= 2; projNum++ {
			storageLimit := memory.GB * memory.Size(projNum)
			bandwidthLimit := memory.GB * memory.Size(projNum*2)
			segmentLimit := int64(1000 * projNum)

			proj := &console.Project{
				ID:             testrand.UUID(),
				Name:           fmt.Sprintf("Project %d", projNum),
				OwnerID:        consoleUser.ID,
				StorageLimit:   &storageLimit,
				BandwidthLimit: &bandwidthLimit,
				SegmentLimit:   &segmentLimit,
			}

			proj, err := consoleDB.Projects().Insert(ctx, proj)
			require.NoError(t, err)
			projects = append(projects, proj)

			_, err = consoleDB.ProjectMembers().Insert(ctx, user.User.ID, proj.ID, console.RoleAdmin)
			require.NoError(t, err)

			bucket, err := sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      testrand.BucketName(),
				ProjectID: proj.ID,
			})
			require.NoError(t, err)

			total := expectedTotal{}

			for objNum := 0; objNum < projNum; objNum++ {
				obj := metabasetest.CreateObject(ctx, t, sat.Metabase.DB, metabase.ObjectStream{
					ProjectID:  proj.ID,
					BucketName: metabase.BucketName(bucket.Name),
					ObjectKey:  metabasetest.RandObjectKey(),
					Version:    12345,
					StreamID:   testrand.UUID(),
				}, byte(16*projNum))

				total.storage += obj.TotalEncryptedSize
				total.segments += int64(obj.SegmentCount)
				total.objects++
			}

			testBandwidth := int64(2000 * projNum)
			err = sat.DB.Orders().
				UpdateBucketBandwidthAllocation(ctx, proj.ID, []byte(bucket.Name), pb.PieceAction_GET, testBandwidth, time.Now())
			require.NoError(t, err)
			total.bandwidth += testBandwidth

			expectedTotals = append(expectedTotals, total)
		}

		sat.Accounting.Tally.Loop.TriggerWait()

		testProjectsFields := func(user *backoffice.UserAccount) {
			sort.Slice(user.Projects, func(i, j int) bool {
				return user.Projects[i].Name < user.Projects[j].Name
			})
			for i, info := range user.Projects {
				proj := projects[i]
				name := proj.Name
				require.Equal(t, proj.PublicID, info.PublicID, name)
				require.Equal(t, name, info.Name, name)
				require.EqualValues(t, *proj.StorageLimit, info.StorageLimit, name)
				require.EqualValues(t, *proj.BandwidthLimit, info.BandwidthLimit, name)
				require.Equal(t, *proj.SegmentLimit, info.SegmentLimit, name)

				total := expectedTotals[i]
				require.Equal(t, total.storage, *info.StorageUsed, name)
				require.Equal(t, total.bandwidth, info.BandwidthUsed, name)
				require.Equal(t, total.segments, *info.SegmentUsed, name)
			}
		}

		user, apiErr = service.GetUserByEmail(ctx, consoleUser.Email)
		require.NoError(t, apiErr.Err)
		require.NotNil(t, user)
		require.Len(t, user.Projects, len(projects))
		testProjectsFields(user)

		user, apiErr = service.GetUser(ctx, consoleUser.ID)
		require.NoError(t, apiErr.Err)
		require.NotNil(t, user)
		require.Len(t, user.Projects, len(projects))
		testProjectsFields(user)
	})
}

func TestSearchUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		consoleDB := sat.DB.Console()

		consoleUser, err := consoleDB.Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "Test User",
			Email:        "test@storj.io",
			Status:       console.Active,
			PasswordHash: make([]byte, 0),
		})
		require.NoError(t, err)
		require.NoError(t, sat.DB.StripeCoinPayments().Customers().Insert(ctx, consoleUser.ID, "cus_random_customer_id"))

		users, apiErr := service.SearchUsers(ctx, consoleUser.Email)
		require.NoError(t, apiErr.Err)
		require.Len(t, users, 1)
		require.Equal(t, consoleUser.ID, users[0].ID)
		require.Equal(t, consoleUser.Status.Info(), users[0].Status)

		users, apiErr = service.SearchUsers(ctx, "test@")
		require.NoError(t, apiErr.Err)
		require.Len(t, users, 1)
		require.Equal(t, consoleUser.ID, users[0].ID)
		require.Equal(t, consoleUser.Status.Info(), users[0].Status)

		// partial name match
		users, apiErr = service.SearchUsers(ctx, "User")
		require.NoError(t, apiErr.Err)
		require.Len(t, users, 1)
		require.Equal(t, consoleUser.ID, users[0].ID)
		require.Equal(t, consoleUser.Status.Info(), users[0].Status)

		require.Equal(t, consoleUser.Status.Info(), users[0].Status)
		users, apiErr = service.SearchUsers(ctx, "nothing")
		require.NoError(t, apiErr.Err)
		require.Empty(t, users)

		// search by ID
		users, apiErr = service.SearchUsers(ctx, consoleUser.ID.String())
		require.NoError(t, apiErr.Err)
		require.Len(t, users, 1)
		require.Equal(t, consoleUser.ID, users[0].ID)
		require.Equal(t, consoleUser.Status.Info(), users[0].Status)

		// searching by invalid ID should return no results
		users, apiErr = service.SearchUsers(ctx, uuid.UUID{}.String())
		require.NoError(t, apiErr.Err)
		require.Empty(t, users)

		customerID, err := consoleDB.Users().GetCustomerID(ctx, consoleUser.ID)
		require.NoError(t, err)
		require.NotEmpty(t, customerID)

		// search by customer ID
		users, apiErr = service.SearchUsers(ctx, customerID)
		require.NoError(t, apiErr.Err)
		require.Len(t, users, 1)
		require.Equal(t, consoleUser.ID, users[0].ID)
		require.Equal(t, consoleUser.Status.Info(), users[0].Status)

		// unknown customer ID returns no results
		users, apiErr = service.SearchUsers(ctx, customerID+"who")
		require.NoError(t, apiErr.Err)
		require.Empty(t, users)

		_, apiErr = service.SearchUsers(ctx, "")
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)
	})
}

func TestUpdateUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Console.Config.FreeTrialDuration = 10 * 24 * time.Hour
				config.Admin.BackOffice.UserGroupsRoleAdmin = []string{"admin"}
				config.Admin.BackOffice.UserGroupsRoleViewer = []string{"viewer"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service

		timeStamp := time.Now().Truncate(time.Hour).UTC()
		service.TestSetNowFn(func() time.Time { return timeStamp })

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User", Email: "test@test.test",
		}, 1)
		require.NoError(t, err)
		user.Status = console.Inactive
		require.NoError(t, sat.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Status: &user.Status}))

		user2, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User", Email: "test2@test.test",
		}, 1)
		require.NoError(t, err)

		limit := int64(100)
		projectLimit := 2
		p, err := sat.AddProject(ctx, user.ID, "Project")
		require.NoError(t, err)
		require.NotEqual(t, limit, p.StorageLimit.Int64())
		require.NotEqual(t, limit, p.BandwidthLimit.Int64())
		require.NotEqual(t, limit, *p.SegmentLimit)

		newName := "new name"
		newUserAgent := "new agent"
		newKind := console.PaidUser
		newStatus := console.Active
		newEmail := "test@test.testing"
		req := backoffice.UpdateUserRequest{
			Name: &newName, Email: &newEmail,
			UserAgent: &newUserAgent,
			Kind:      &newKind, Status: &newStatus,
			ProjectLimit: &projectLimit, StorageLimit: &limit,
			BandwidthLimit: &limit, SegmentLimit: &limit,
		}

		testFailAuth := func(groups []string) {
			_, apiErr := service.UpdateUser(ctx, &backoffice.AuthInfo{Groups: groups}, user.ID, req)
			require.True(t, apiErr.Status == http.StatusUnauthorized || apiErr.Status == http.StatusForbidden)
			require.Error(t, apiErr.Err)
			require.Contains(t, apiErr.Err.Error(), "not authorized")
		}

		testFailAuth(nil)
		testFailAuth([]string{})
		testFailAuth([]string{"viewer"}) // insufficient permissions

		authInfo := &backoffice.AuthInfo{Groups: []string{"admin"}}

		_, apiErr := service.UpdateUser(ctx, authInfo, testrand.UUID(), req)
		require.Equal(t, http.StatusNotFound, apiErr.Status)
		require.Error(t, apiErr.Err)

		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "reason is required")

		req.Reason = "reason"
		u, apiErr := service.UpdateUser(ctx, authInfo, user.ID, req)
		require.NoError(t, apiErr.Err)
		require.Equal(t, newName, u.FullName)
		require.Equal(t, newEmail, u.Email)
		require.Equal(t, newUserAgent, u.UserAgent)
		require.Equal(t, newKind.Info(), u.Kind)
		require.NotNil(t, u.UpgradeTime)
		require.WithinDuration(t, timeStamp, *u.UpgradeTime, time.Second)
		require.Equal(t, newStatus.Info(), u.Status)
		// since we provided custom limits, paid kind defaults are ignored
		require.Equal(t, projectLimit, u.ProjectLimit)
		require.Equal(t, limit, u.StorageLimit)
		require.Equal(t, limit, u.BandwidthLimit)
		require.Equal(t, limit, u.SegmentLimit)

		p, err = sat.DB.Console().Projects().Get(ctx, p.ID)
		require.NoError(t, err)
		require.Equal(t, limit, p.StorageLimit.Int64())
		require.Equal(t, limit, p.BandwidthLimit.Int64())
		require.Equal(t, limit, *p.SegmentLimit)

		// test setting default paid or NFR limits
		usageLimits := sat.Config.Console.UsageLimits
		newKind = console.NFRUser
		req = backoffice.UpdateUserRequest{Kind: &newKind, Reason: "reason"}
		u, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.NoError(t, apiErr.Err)
		require.Equal(t, usageLimits.Project.Nfr, u.ProjectLimit)
		require.Equal(t, usageLimits.Storage.Nfr.Int64(), u.StorageLimit)
		require.Equal(t, usageLimits.Bandwidth.Nfr.Int64(), u.BandwidthLimit)
		require.Equal(t, usageLimits.Segment.Nfr, u.SegmentLimit)

		p, err = sat.DB.Console().Projects().Get(ctx, p.ID)
		require.NoError(t, err)
		require.Equal(t, usageLimits.Storage.Nfr.Int64(), p.StorageLimit.Int64())
		require.Equal(t, usageLimits.Bandwidth.Nfr.Int64(), p.BandwidthLimit.Int64())
		require.Equal(t, usageLimits.Segment.Nfr, *p.SegmentLimit)

		newKind = console.PaidUser
		req = backoffice.UpdateUserRequest{Kind: &newKind, Reason: "reason"}
		u, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.NoError(t, apiErr.Err)
		require.Equal(t, usageLimits.Project.Paid, u.ProjectLimit)
		require.Equal(t, usageLimits.Storage.Paid.Int64(), u.StorageLimit)
		require.Equal(t, usageLimits.Bandwidth.Paid.Int64(), u.BandwidthLimit)
		require.Equal(t, usageLimits.Segment.Paid, u.SegmentLimit)

		// trial expiration
		require.Nil(t, u.TrialExpiration)
		newKind = console.FreeUser
		req = backoffice.UpdateUserRequest{Kind: &newKind, Reason: "reason"}
		u, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.NoError(t, apiErr.Err)
		require.NotNil(t, u.TrialExpiration)
		require.WithinDuration(t, *u.TrialExpiration, timeStamp, sat.Config.Console.Config.FreeTrialDuration+time.Minute)

		req.TrialExpiration = new(string)
		*req.TrialExpiration = "" // remove trial expiration
		u, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.NoError(t, apiErr.Err)
		require.Nil(t, u.TrialExpiration)

		*req.TrialExpiration = timeStamp.Add(24 * time.Hour).Format(time.RFC3339)
		u, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.NoError(t, apiErr.Err)
		require.NotNil(t, u.TrialExpiration)
		require.WithinDuration(t, *u.TrialExpiration, timeStamp, 24*time.Hour)

		// validation
		newKind = console.UserKind(100)
		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, backoffice.UpdateUserRequest{Kind: &newKind,
			Reason: "reason",
		})
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)

		newStatus = console.UserStatus(100)
		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, backoffice.UpdateUserRequest{Status: &newStatus,
			Reason: "reason",
		})
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)

		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, backoffice.UpdateUserRequest{Email: &user2.Email,
			Reason: "reason",
		})
		require.Equal(t, http.StatusConflict, apiErr.Status)
		require.Error(t, apiErr.Err)

		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, backoffice.UpdateUserRequest{
			Name:   new(string),
			Reason: "reason",
		})
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)

		req = backoffice.UpdateUserRequest{Reason: "reason", TrialExpiration: new(string)}
		*req.TrialExpiration = timeStamp.Add(-24 * time.Hour).Format(time.RFC3339)
		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "must be in the future")

		*req.TrialExpiration = "invalid"
		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "invalid trial expiration format")

		*req.TrialExpiration = timeStamp.Add(254 * time.Hour).Format(time.RFC3339)
		newKind = console.PaidUser
		req.Kind = &newKind
		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "for free users")

		// make user paid again
		u, apiErr = service.UpdateUser(ctx, authInfo, user.ID, backoffice.UpdateUserRequest{Kind: &newKind,
			Reason: "reason",
		})
		require.NoError(t, apiErr.Err)
		require.Equal(t, console.PaidUser.Info(), u.Kind)
		require.Nil(t, u.TrialExpiration)

		*req.TrialExpiration = timeStamp.Add(254 * time.Hour).Format(time.RFC3339)
		req.Kind = nil
		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "for free users")

		limit = -1
		req.StorageLimit = &limit
		req.BandwidthLimit = &limit
		req.SegmentLimit = &limit
		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.Error(t, apiErr.Err)
		require.Equal(t, http.StatusBadRequest, apiErr.Status)

		// test MemberUser kind.
		newKind = console.MemberUser
		req = backoffice.UpdateUserRequest{Kind: &newKind, Reason: "reason"}
		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.Equal(t, http.StatusForbidden, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "cannot change to member user while having active projects")

		err = sat.DB.Console().Projects().Delete(ctx, p.ID)
		require.NoError(t, err)

		u, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.NoError(t, apiErr.Err)
		require.Equal(t, console.MemberUser.Info(), u.Kind)
		require.Nil(t, u.TrialExpiration)

		// test setting status to pending deletion fails
		newStatus = console.PendingDeletion
		req.Status = &newStatus
		req.Kind = nil
		_, apiErr = service.UpdateUser(ctx, authInfo, user.ID, req)
		require.Equal(t, http.StatusForbidden, apiErr.Status)
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "not authorized to set user status to pending deletion")
	})
}

func TestDisableUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.BackOffice.UserGroupsRoleAdmin = []string{"admin"}
				config.Admin.BackOffice.UserGroupsRoleViewer = []string{"viewer"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service

		t.Run("authorization", func(t *testing.T) {
			req := backoffice.DisableUserRequest{Reason: "reason"}
			testFailAuth := func(groups []string) {
				_, apiErr := service.DisableUser(ctx, &backoffice.AuthInfo{Groups: groups}, testrand.UUID(), req)
				require.True(t, apiErr.Status == http.StatusUnauthorized || apiErr.Status == http.StatusForbidden)
				require.Error(t, apiErr.Err)
				require.Contains(t, apiErr.Err.Error(), "not authorized")
			}

			testFailAuth(nil)
			testFailAuth([]string{})
			testFailAuth([]string{"viewer"}) // insufficient permissions
			req.SetPendingDeletion = true
			testFailAuth([]string{"viewer"}) // insufficient permissions
			req.SetPendingDeletion = false

			authInfo := &backoffice.AuthInfo{Groups: []string{"admin"}}

			_, apiErr := service.DisableUser(ctx, authInfo, testrand.UUID(), req)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
			require.Error(t, apiErr.Err)

			req.SetPendingDeletion = true
			_, apiErr = service.DisableUser(ctx, authInfo, testrand.UUID(), req)
			require.Equal(t, http.StatusConflict, apiErr.Status)
			require.Error(t, apiErr.Err)
			require.Contains(t, apiErr.Err.Error(), "pending deletion is not enabled")

			service.TestToggleAbbreviatedUserDelete(true)
			defer service.TestToggleAbbreviatedUserDelete(false)

			_, apiErr = service.DisableUser(ctx, authInfo, testrand.UUID(), req)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
			require.Error(t, apiErr.Err)
		})

		t.Run("full disable flow", func(t *testing.T) {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User", Email: "test@test.test",
			}, 1)
			require.NoError(t, err)

			p, err := sat.AddProject(ctx, user.ID, "Project")
			require.NoError(t, err)

			authInfo := &backoffice.AuthInfo{Groups: []string{"admin"}}

			_, apiErr := service.DisableUser(ctx, authInfo, testrand.UUID(), backoffice.DisableUserRequest{})
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Error(t, apiErr.Err)
			require.Contains(t, apiErr.Err.Error(), "reason is required")

			request := backoffice.DisableUserRequest{Reason: "reason"}
			_, apiErr = service.DisableUser(ctx, authInfo, testrand.UUID(), request)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
			require.Error(t, apiErr.Err)

			_, apiErr = service.DisableUser(ctx, authInfo, user.ID, request)
			require.Equal(t, http.StatusConflict, apiErr.Status)
			require.Error(t, apiErr.Err)
			require.Contains(t, apiErr.Err.Error(), "active projects")

			err = sat.DB.Console().Projects().Delete(ctx, p.ID)
			require.NoError(t, err)

			inv, err := sat.Admin.Payments.Accounts.Invoices().Create(ctx, user.ID, 1000, "test invoice 1")
			require.NoError(t, err)

			_, apiErr = service.DisableUser(ctx, authInfo, user.ID, request)
			require.Equal(t, http.StatusConflict, apiErr.Status)
			require.Error(t, apiErr.Err)
			require.Contains(t, apiErr.Err.Error(), "unpaid invoices")

			_, err = sat.Admin.Payments.Accounts.Invoices().Delete(ctx, inv.ID)
			require.NoError(t, err)

			u, apiErr := service.DisableUser(ctx, authInfo, user.ID, request)
			require.NoError(t, apiErr.Err)
			require.Contains(t, u.Email, "deactivated")
			require.Equal(t, console.Deleted, u.Status.Value)
		})

		t.Run("abbreviated disable flow", func(t *testing.T) {
			service.TestToggleAbbreviatedUserDelete(true)
			defer service.TestToggleAbbreviatedUserDelete(false)

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User", Email: "test@test.test",
			}, 1)
			require.NoError(t, err)

			authInfo := &backoffice.AuthInfo{Groups: []string{"admin"}}

			inv, err := sat.Admin.Payments.Accounts.Invoices().Create(ctx, user.ID, 1000, "test invoice 1")
			require.NoError(t, err)

			request := backoffice.DisableUserRequest{Reason: "reason", SetPendingDeletion: true}

			_, apiErr := service.DisableUser(ctx, authInfo, user.ID, request)
			require.Equal(t, http.StatusConflict, apiErr.Status)
			require.Error(t, apiErr.Err)
			// invoices are still checked in the abbreviated flow
			require.Contains(t, apiErr.Err.Error(), "unpaid invoices")

			_, err = sat.Admin.Payments.Accounts.Invoices().Delete(ctx, inv.ID)
			require.NoError(t, err)

			_, err = sat.AddProject(ctx, user.ID, "Project")
			require.NoError(t, err)

			// projects existing will not block abbreviated deletion
			u, apiErr := service.DisableUser(ctx, authInfo, user.ID, request)
			require.NoError(t, apiErr.Err)
			require.Equal(t, console.PendingDeletion, u.Status.Value)
		})
	})
}

func TestDisableMFA(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User", Email: "test@test.io",
		}, 1)
		require.NoError(t, err)

		// Enable MFA.
		user.MFAEnabled = true
		user.MFASecretKey = "randomtext"
		user.MFARecoveryCodes = []string{"0123456789"}
		secretKeyPtr := &user.MFASecretKey
		err = sat.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
			MFAEnabled:       &user.MFAEnabled,
			MFASecretKey:     &secretKeyPtr,
			MFARecoveryCodes: &user.MFARecoveryCodes,
		})
		require.NoError(t, err)

		u, apiErr := service.GetUser(ctx, user.ID)
		require.NoError(t, apiErr.Err)
		require.True(t, u.MFAEnabled)

		user, err = sat.DB.Console().Users().Get(ctx, u.ID)
		require.NoError(t, err)
		require.True(t, user.MFAEnabled)
		require.NotEmpty(t, user.MFASecretKey)
		require.NotEmpty(t, user.MFARecoveryCodes)

		authInfo := &backoffice.AuthInfo{Email: "test@example.com"}

		apiErr = service.ToggleMFA(ctx, authInfo, user.ID, backoffice.ToggleMfaRequest{})
		require.Error(t, apiErr.Err)
		require.Contains(t, apiErr.Err.Error(), "reason is required")

		apiErr = service.ToggleMFA(ctx, authInfo, user.ID, backoffice.ToggleMfaRequest{Reason: "reason"})
		require.NoError(t, apiErr.Err)

		u, apiErr = service.GetUser(ctx, user.ID)
		require.NoError(t, apiErr.Err)
		require.False(t, u.MFAEnabled)

		user, err = sat.DB.Console().Users().Get(ctx, u.ID)
		require.NoError(t, err)
		require.False(t, user.MFAEnabled)
		require.Empty(t, user.MFASecretKey)
		require.Empty(t, user.MFARecoveryCodes)
	})
}

func TestCreateRestKey(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User", Email: "test@test.io",
		}, 1)
		require.NoError(t, err)

		request := backoffice.CreateRestKeyRequest{
			Expiration: time.Now().Add(24 * time.Hour),
			Reason:     "reason",
		}

		authInfo := &backoffice.AuthInfo{Email: "test@example.com"}

		t.Run("Success - create REST key with expiration", func(t *testing.T) {
			key, apiErr := service.CreateRestKey(ctx, authInfo, user.ID, request)
			require.NoError(t, apiErr.Err)
			require.NotNil(t, key)
		})

		t.Run("Error - user not found", func(t *testing.T) {
			_, apiErr := service.CreateRestKey(ctx, authInfo, testrand.UUID(), request)
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("Error - missing expiration", func(t *testing.T) {
			request = backoffice.CreateRestKeyRequest{}
			_, apiErr := service.CreateRestKey(ctx, authInfo, user.ID, request)
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Contains(t, apiErr.Err.Error(), "expiration is required")
		})

		t.Run("Error - expiration in the past", func(t *testing.T) {
			request.Expiration = time.Now().Add(-1 * time.Hour)
			_, apiErr := service.CreateRestKey(ctx, authInfo, user.ID, request)
			require.Error(t, apiErr.Err)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Contains(t, apiErr.Err.Error(), "expiration must be in the future")
		})

		t.Run("Error - missing reason", func(t *testing.T) {
			_, apiErr := service.CreateRestKey(ctx, authInfo, user.ID, backoffice.CreateRestKeyRequest{
				Expiration: time.Now().Add(24 * time.Hour),
			})
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
			require.Error(t, apiErr.Err)
			require.Contains(t, apiErr.Err.Error(), "reason is required")
		})
	})
}
