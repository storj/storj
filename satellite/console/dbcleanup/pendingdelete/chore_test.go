// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package pendingdelete_test

import (
	"fmt"
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
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/uplink"
)

func TestPendingDeleteChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.PendingDeleteCleanup.Enabled = true
				config.PendingDeleteCleanup.Project.Enabled = true
				config.PendingDeleteCleanup.Project.BufferTime = time.Hour
				config.PendingDeleteCleanup.User.Enabled = true
				config.PendingDeleteCleanup.User.BufferTime = time.Hour
				config.PendingDeleteCleanup.TrialFreeze.Enabled = true
				config.PendingDeleteCleanup.TrialFreeze.BufferTime = time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]
		chore := sat.Core.ConsoleDBCleanup.PendingDeleteChore
		projectsDB := sat.DB.Console().Projects()
		accFreezeDB := sat.DB.Console().AccountFreezeEvents()
		usersDB := sat.DB.Console().Users()

		chore.Loop.Pause()

		// delete existing project to start fresh
		err := projectsDB.Delete(ctx, upl.Projects[0].ID)
		require.NoError(t, err)

		now := time.Now().Truncate(time.Minute)
		projectsDB.TestSetNowFn(func() time.Time { return now })
		chore.TestSetNowFn(func() time.Time { return now })

		uploadData := func(projID uuid.UUID, userID uuid.UUID) {
			uCtx, err := sat.UserContext(ctx, userID)
			require.NoError(t, err)
			_, apiKey, err := sat.API.Console.Service.CreateAPIKey(
				uCtx, projID, "root", macaroon.APIKeyVersionMin,
			)
			require.NoError(t, err)
			access, err := upl.Config.RequestAccessWithPassphrase(ctx, sat.URL(), apiKey.Serialize(), "")
			require.NoError(t, err)
			projectUplink, err := uplink.OpenProject(ctx, access)
			require.NoError(t, err)
			_, err = projectUplink.EnsureBucket(ctx, "test-bucket")
			require.NoError(t, err)
			upload, err := projectUplink.UploadObject(ctx, "test-bucket", "test-object", nil)
			require.NoError(t, err)
			_, err = upload.Write(testrand.Bytes(14 * memory.KiB))
			require.NoError(t, err)
			require.NoError(t, upload.Commit())
		}

		user, err := usersDB.GetByEmailAndTenant(ctx, upl.User[sat.ID()].Email, nil)
		require.NoError(t, err)
		// Create a project pending deletion
		projectForDeletion, err := sat.AddProject(ctx, user.ID, "project-for-deletion")
		require.NoError(t, err)
		uploadData(projectForDeletion.ID, user.ID)
		err = projectsDB.UpdateStatus(ctx, projectForDeletion.ID, console.ProjectPendingDeletion)
		require.NoError(t, err)

		// Create a frozen user pending deletion
		frozenUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName:  "frozen_user",
			ShortName: "",
			Email:     "frozen@test.test",
		}, 1)
		require.NoError(t, err)
		pd := console.PendingDeletion
		err = usersDB.Update(ctx, frozenUser.ID, console.UpdateUserRequest{
			Status: &pd,
		})
		require.NoError(t, err)
		frozenProject, err := sat.AddProject(ctx, frozenUser.ID, "frozen-project")
		require.NoError(t, err)
		uploadData(frozenProject.ID, frozenUser.ID)
		_, err = accFreezeDB.Upsert(ctx, &console.AccountFreezeEvent{
			UserID:             frozenUser.ID,
			Type:               console.TrialExpirationFreeze,
			DaysTillEscalation: nil,
		})
		require.NoError(t, err)

		// create a user pending deletion but not frozen
		pendingDeletionUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName:  "pending_deletion_user",
			ShortName: "",
			Email:     "pending@test.test",
		}, 1)
		require.NoError(t, err)
		require.NoError(t, usersDB.Update(ctx, pendingDeletionUser.ID, console.UpdateUserRequest{Status: &pd}))
		pdUserProject, err := sat.AddProject(ctx, pendingDeletionUser.ID, "pending-deletion-user-project")
		require.NoError(t, err)
		uploadData(pdUserProject.ID, pendingDeletionUser.ID)

		// Verify all users have data initially
		objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 3)

		// Run chore before buffer time - should not delete anything
		chore.Loop.TriggerWait()

		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 3)

		// Move past buffer time and run chore - should delete all data
		chore.TestSetNowFn(func() time.Time {
			return now.Add(time.Hour + 10*time.Minute)
		})
		chore.Loop.TriggerWait()

		// Verify all objects are deleted
		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Empty(t, objects)

		// Verify pending deletion project is disabled
		p, err := projectsDB.Get(ctx, projectForDeletion.ID)
		require.NoError(t, err)
		require.NotNil(t, p.Status)
		require.Equal(t, console.ProjectDisabled, *p.Status)
		// verify owner of this project is not deleted
		u, err := usersDB.Get(ctx, p.OwnerID)
		require.NoError(t, err)
		require.Equal(t, console.Active, u.Status)

		// Verify pending deletion users are deleted
		u, err = usersDB.Get(ctx, frozenUser.ID)
		require.NoError(t, err)
		require.Equal(t, console.Deleted, u.Status)

		u, err = usersDB.Get(ctx, pendingDeletionUser.ID)
		require.NoError(t, err)
		require.Equal(t, console.Deleted, u.Status)

		// verify their projects are disabled
		projects, err := projectsDB.GetOwn(ctx, frozenUser.ID)
		require.NoError(t, err)
		require.Len(t, projects, 1)
		require.Equal(t, console.ProjectDisabled, *projects[0].Status)

		projects, err = projectsDB.GetOwn(ctx, pendingDeletionUser.ID)
		require.NoError(t, err)
		require.Len(t, projects, 1)
		require.Equal(t, console.ProjectDisabled, *projects[0].Status)
	})
}

func TestPendingDeleteChore_PendingDeletionProjects(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.PendingDeleteCleanup.Enabled = true
				config.PendingDeleteCleanup.Project.Enabled = true
				config.PendingDeleteCleanup.Project.BufferTime = time.Hour
				config.PendingDeleteCleanup.ListLimit = 2 // small limit to test batching
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]
		chore := sat.Core.ConsoleDBCleanup.PendingDeleteChore
		projectsDB := sat.DB.Console().Projects()
		usersDB := sat.DB.Console().Users()
		domainsDB := sat.DB.Console().Domains()

		entitlementsService := entitlements.NewService(testplanet.NewLogger(t), sat.DB.Console().Entitlements())

		chore.Loop.Pause()

		user, err := usersDB.GetByEmailAndTenant(ctx, upl.User[sat.ID()].Email, nil)
		require.NoError(t, err)
		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		err = projectsDB.Delete(ctx, upl.Projects[0].ID)
		require.NoError(t, err)

		projectsCount := 4
		objectsCount := 4

		now := time.Now().Truncate(time.Minute)
		projectsDB.TestSetNowFn(func() time.Time { return now })
		chore.TestSetNowFn(func() time.Time { return now })

		addProjectAndData := func(status console.ProjectStatus, hasObjectLock bool) uuid.UUID {
			p, err := sat.AddProject(ctx, user.ID, "new-project")
			require.NoError(t, err)
			require.NotNil(t, p)

			_, apiKey, err := sat.API.Console.Service.CreateAPIKey(
				userCtx, p.ID, "root", macaroon.APIKeyVersionObjectLock,
			)
			require.NoError(t, err)

			access, err := upl.Config.RequestAccessWithPassphrase(ctx, sat.URL(), apiKey.Serialize(), "")
			require.NoError(t, err)

			uplProject, err := uplink.OpenProject(ctx, access)
			require.NoError(t, err)

			_, err = uplProject.EnsureBucket(ctx, "test-bucket")
			require.NoError(t, err)

			if hasObjectLock {
				_, err = sat.DB.Buckets().UpdateBucketObjectLockSettings(ctx, buckets.UpdateBucketObjectLockParams{
					ObjectLockEnabled: true,
					ProjectID:         p.ID,
					Name:              "test-bucket",
				})
				require.NoError(t, err)
			}

			for j := range objectsCount {
				upload, err := uplProject.UploadObject(ctx, "test-bucket", fmt.Sprintf("test-object-%d", j), nil)
				require.NoError(t, err)
				_, err = upload.Write(testrand.Bytes(14 * memory.KiB))
				require.NoError(t, err)
				require.NoError(t, upload.Commit())
			}

			err = entitlementsService.Projects().SetNewBucketPlacementsByPublicID(ctx, p.PublicID, []storj.PlacementConstraint{1})
			require.NoError(t, err)
			_, err = domainsDB.Create(ctx, console.Domain{ProjectID: p.ID, Subdomain: p.Name, CreatedBy: user.ID})
			require.NoError(t, err)

			if status != console.ProjectActive {
				err = projectsDB.UpdateStatus(ctx, p.ID, status)
				require.NoError(t, err)
			}

			return p.ID
		}

		projectsMarkedForDeletion := make([]uuid.UUID, 0)
		activeProjects := make([]uuid.UUID, 0)

		for i := range projectsCount {
			if i%2 == 0 {
				projectsMarkedForDeletion = append(projectsMarkedForDeletion, addProjectAndData(console.ProjectPendingDeletion, false))
				continue
			}
			activeProjects = append(activeProjects, addProjectAndData(console.ProjectActive, false))
		}

		// Verify that all four projects have objects uploaded
		objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, projectsCount*objectsCount)

		chore.Loop.TriggerWait()

		testObjectsLength := func(projectID uuid.UUID, expected int) {
			objs, err := sat.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)

			pObjs := make([]metabase.Object, 0)
			for i, object := range objs {
				if projectID == object.ProjectID {
					pObjs = append(pObjs, objs[i])
				}
			}

			require.Len(t, pObjs, expected)
		}

		verifyHasDbData := func(projID uuid.UUID, hasData bool) {
			p, err := projectsDB.Get(ctx, projID)
			require.NoError(t, err)
			require.NotNil(t, p)

			domains, err := domainsDB.GetAllDomainNamesByProjectID(ctx, projID)
			require.NoError(t, err)
			if !hasData {
				require.Empty(t, domains)
			} else {
				require.NotEmpty(t, domains)
			}

			feats, err := entitlementsService.Projects().GetByPublicID(ctx, p.PublicID)
			if !hasData {
				require.Error(t, err)
				require.True(t, entitlements.ErrNotFound.Has(err))
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, feats.NewBucketPlacements)
			}
		}

		// verify that all users have data after the first chore run,
		// even those marked for deletion because the buffer time has not yet elapsed.
		for _, projectID := range projectsMarkedForDeletion {
			testObjectsLength(projectID, objectsCount)
			verifyHasDbData(projectID, true)
		}
		for _, projectID := range activeProjects {
			testObjectsLength(projectID, objectsCount)
			verifyHasDbData(projectID, true)
		}

		chore.TestSetNowFn(func() time.Time {
			// set the chore time to some time beyond the buffer time
			return now.Add(sat.Config.PendingDeleteCleanup.Project.BufferTime + (24 * time.Hour))
		})
		chore.Loop.TriggerWait()

		// Verify that all objects are deleted for projects marked for deletion
		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, (projectsCount*objectsCount)/2)

		testDisabled := func(projectID uuid.UUID) {
			p, err := projectsDB.Get(ctx, projectID)
			require.NoError(t, err)
			require.NotNil(t, p.Status)
			require.Equal(t, console.ProjectDisabled, *p.Status)
		}

		for _, projectID := range projectsMarkedForDeletion {
			// verify that marked projects have no objects and
			// are disabled.
			testObjectsLength(projectID, 0)
			verifyHasDbData(projectID, false)
			testDisabled(projectID)
		}
		for _, projectID := range activeProjects {
			// verify that the user has objects
			testObjectsLength(projectID, objectsCount)
			verifyHasDbData(projectID, true)
		}

		// test that deletion is successful when concurrent delete is enabled
		chore.TestSetDeleteConcurrency(2)

		newProjectsList := make([]uuid.UUID, 0)
		for range projectsCount {
			newProjectsList = append(newProjectsList, addProjectAndData(console.ProjectPendingDeletion, false))
		}

		// mark active projects for deletion
		for i, projectID := range activeProjects {
			projectsDB.TestSetNowFn(func() time.Time { return now.Add(time.Duration(i) * time.Minute) })
			err = projectsDB.UpdateStatus(ctx, projectID, console.ProjectPendingDeletion)
			require.NoError(t, err)
		}

		newProjectsList = append(newProjectsList, activeProjects...)

		chore.Loop.TriggerWait()

		// Verify that all objects are deleted for projects marked for deletion
		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Empty(t, objects)

		for _, projectID := range newProjectsList {
			testObjectsLength(projectID, 0)
			verifyHasDbData(projectID, false)
			testDisabled(projectID)
		}

		// test that a project that contains a bucket with object lock enabled is skipped and not deleted
		objectLockProjectID := addProjectAndData(console.ProjectPendingDeletion, true)

		chore.Loop.TriggerWait()

		// verify that the project still has its objects and is still pending deletion
		p, err := projectsDB.Get(ctx, objectLockProjectID)
		require.NoError(t, err)
		require.Equal(t, console.ProjectPendingDeletion, *p.Status)
		testObjectsLength(objectLockProjectID, objectsCount)
	})
}

func TestPendingDeleteChore_FrozenUsers(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.PendingDeleteCleanup.Enabled = true
				config.PendingDeleteCleanup.TrialFreeze.Enabled = true
				config.PendingDeleteCleanup.TrialFreeze.BufferTime = time.Hour
				config.PendingDeleteCleanup.BillingFreeze.Enabled = true
				config.PendingDeleteCleanup.BillingFreeze.BufferTime = time.Hour
				config.PendingDeleteCleanup.ListLimit = 2 // small limit to test batching
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]
		chore := sat.Core.ConsoleDBCleanup.PendingDeleteChore
		accFreezeDB := sat.DB.Console().AccountFreezeEvents()
		usersDB := sat.DB.Console().Users()

		chore.Loop.Pause()

		type projectAndUser struct {
			projectID uuid.UUID
			userID    uuid.UUID
		}

		usersCount := 4
		userObjectsCount := 4

		addUserAndData := func(email string, freezeType *console.AccountFreezeEventType) projectAndUser {
			u, err := sat.AddUser(ctx, console.CreateUser{
				FullName:  "test_name",
				ShortName: "",
				Email:     email,
			}, 1)
			require.NoError(t, err)

			p, err := sat.AddProject(ctx, u.ID, "new project")
			require.NoError(t, err)

			userCtx, err := sat.UserContext(ctx, u.ID)
			require.NoError(t, err)
			_, apiKey, err := sat.API.Console.Service.CreateAPIKey(
				userCtx, p.ID, "root", macaroon.APIKeyVersionMin,
			)
			require.NoError(t, err)

			access, err := upl.Config.RequestAccessWithPassphrase(ctx, sat.URL(), apiKey.Serialize(), "")
			require.NoError(t, err)

			uplProject, err := uplink.OpenProject(ctx, access)
			require.NoError(t, err)

			_, err = uplProject.EnsureBucket(ctx, "test-bucket")
			require.NoError(t, err)

			for j := range userObjectsCount {
				upload, err := uplProject.UploadObject(ctx, "test-bucket", fmt.Sprintf("test-object-%d", j), nil)
				require.NoError(t, err)
				_, err = upload.Write(testrand.Bytes(14 * memory.KiB))
				require.NoError(t, err)
				require.NoError(t, upload.Commit())
			}

			if freezeType != nil {
				// insert freeze event for user
				_, err = accFreezeDB.Upsert(ctx, &console.AccountFreezeEvent{
					UserID:             u.ID,
					Type:               *freezeType,
					DaysTillEscalation: nil,
				})
				require.NoError(t, err)

				// mark the user as pending deletion
				pd := console.PendingDeletion
				err = usersDB.Update(ctx, u.ID, console.UpdateUserRequest{
					Status: &pd,
				})
				require.NoError(t, err)

				u, err = usersDB.Get(ctx, u.ID)
				require.NoError(t, err)
				require.Equal(t, console.PendingDeletion, u.Status)
			}

			return projectAndUser{userID: u.ID, projectID: p.ID}
		}

		usersMarkedForDeletion := make([]projectAndUser, 0)
		activeUsers := make([]projectAndUser, 0)

		for i := range usersCount {
			eventType := console.TrialExpirationFreeze
			if i == 0 {
				usersMarkedForDeletion = append(usersMarkedForDeletion, addUserAndData(fmt.Sprintf("test%d@storj.test", i), &eventType))
			} else if i == usersCount-1 {
				eventType = console.BillingFreeze
				usersMarkedForDeletion = append(usersMarkedForDeletion, addUserAndData(fmt.Sprintf("test%d@storj.test", i), &eventType))
			} else {
				activeUsers = append(activeUsers, addUserAndData(fmt.Sprintf("test%d@storj.test", i), nil))
			}
		}

		// Verify that all four users have objects uploaded
		objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, usersCount*userObjectsCount)

		chore.Loop.TriggerWait()

		testObjectsLength := func(usr projectAndUser, expected int) {
			usrObjs := make([]metabase.Object, 0)
			objs, err := sat.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)

			for i, object := range objs {
				if usr.projectID == object.ProjectID {
					usrObjs = append(usrObjs, objs[i])
				}
			}

			require.Len(t, usrObjs, expected)
		}

		// verify that all users have objects after the first chore run,
		// even those marked for deletion because the buffer time has not yet elapsed.
		for _, user := range usersMarkedForDeletion {
			// verify that the user has objects
			testObjectsLength(user, userObjectsCount)
		}
		for _, user := range activeUsers {
			// verify that the user has objects
			testObjectsLength(user, userObjectsCount)
		}

		chore.TestSetNowFn(func() time.Time {
			// set the chore time to some time beyond the escalation buffer time
			return time.Now().Add(sat.Config.PendingDeleteCleanup.BillingFreeze.BufferTime + time.Hour)
		})
		chore.Loop.TriggerWait()

		// Verify that all objects are deleted for users marked for deletion
		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, (usersCount*userObjectsCount)/2)

		testDeactivated := func(user projectAndUser) {
			u, err := usersDB.Get(ctx, user.userID)
			require.NoError(t, err)
			require.Equal(t, console.Deleted, u.Status)

			// list all projects for the user, they should be deactivated
			projects, err := sat.DB.Console().Projects().GetOwn(ctx, user.userID)
			require.NoError(t, err)
			for _, p := range projects {
				require.NotNil(t, p.Status)
				require.Equal(t, console.ProjectDisabled, *p.Status)
			}
		}

		for _, user := range usersMarkedForDeletion {
			// verify that deleted user has no objects
			testObjectsLength(user, 0)
			testDeactivated(user)
		}
		for _, user := range activeUsers {
			// verify that the user has objects
			testObjectsLength(user, userObjectsCount)
		}

		// test that deletion is successful when concurrent delete is enabled

		chore.TestSetDeleteConcurrency(2)

		newUserList := make([]projectAndUser, 0)
		// add some frozen users with more data
		for i := range usersCount {
			eventType := console.TrialExpirationFreeze
			if i%2 == 0 {
				eventType = console.BillingFreeze
			}
			newUserList = append(newUserList, addUserAndData(fmt.Sprintf("deleted+%d@test.test", i), &eventType))
		}

		// freeze and escalate active users
		for _, u := range activeUsers {
			_, err = accFreezeDB.Upsert(ctx, &console.AccountFreezeEvent{
				UserID:             u.userID,
				Type:               console.TrialExpirationFreeze,
				DaysTillEscalation: nil,
			})
			require.NoError(t, err)

			// mark the user as pending deletion
			pD := console.PendingDeletion
			err = usersDB.Update(ctx, u.userID, console.UpdateUserRequest{
				Status: &pD,
			})
			require.NoError(t, err)
		}

		newUserList = append(newUserList, activeUsers...)

		chore.Loop.TriggerWait()

		// no objects should be left
		// Verify that all objects are deleted for users marked for deletion
		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Empty(t, objects)

		for _, user := range newUserList {
			testObjectsLength(user, 0)
			testDeactivated(user)
		}
	})
}

func TestPendingDeleteChore_PendingDeletionUsers(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.PendingDeleteCleanup.Enabled = true
				config.PendingDeleteCleanup.User.Enabled = true
				config.PendingDeleteCleanup.User.BufferTime = time.Hour
				config.PendingDeleteCleanup.ListLimit = 1 // small limit to test batching
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]
		chore := sat.Core.ConsoleDBCleanup.PendingDeleteChore
		accFreezeDB := sat.DB.Console().AccountFreezeEvents()
		usersDB := sat.DB.Console().Users()

		chore.Loop.Pause()

		type projectAndUser struct {
			projectID uuid.UUID
			userID    uuid.UUID
		}

		userObjectsCount := 4

		addUserAndData := func(email string, pendingDeletion, frozen bool) projectAndUser {
			u, err := sat.AddUser(ctx, console.CreateUser{
				FullName:  "test_name",
				ShortName: "",
				Email:     email,
			}, 1)
			require.NoError(t, err)

			p, err := sat.AddProject(ctx, u.ID, "new project")
			require.NoError(t, err)

			userCtx, err := sat.UserContext(ctx, u.ID)
			require.NoError(t, err)
			_, apiKey, err := sat.API.Console.Service.CreateAPIKey(
				userCtx, p.ID, "root", macaroon.APIKeyVersionMin,
			)
			require.NoError(t, err)

			access, err := upl.Config.RequestAccessWithPassphrase(ctx, sat.URL(), apiKey.Serialize(), "")
			require.NoError(t, err)

			uplProject, err := uplink.OpenProject(ctx, access)
			require.NoError(t, err)

			_, err = uplProject.EnsureBucket(ctx, "test-bucket")
			require.NoError(t, err)

			for j := range userObjectsCount {
				upload, err := uplProject.UploadObject(ctx, "test-bucket", fmt.Sprintf("test-object-%d", j), nil)
				require.NoError(t, err)
				_, err = upload.Write(testrand.Bytes(14 * memory.KiB))
				require.NoError(t, err)
				require.NoError(t, upload.Commit())
			}

			if pendingDeletion {
				if frozen {
					// insert freeze event for user
					_, err = accFreezeDB.Upsert(ctx, &console.AccountFreezeEvent{
						UserID:             u.ID,
						Type:               console.BillingFreeze,
						DaysTillEscalation: nil,
					})
					require.NoError(t, err)
				}

				// mark the user as pending deletion
				pd := console.PendingDeletion
				err = usersDB.Update(ctx, u.ID, console.UpdateUserRequest{
					Status: &pd,
				})
				require.NoError(t, err)

				u, err = usersDB.Get(ctx, u.ID)
				require.NoError(t, err)
				require.Equal(t, console.PendingDeletion, u.Status)
			}

			return projectAndUser{userID: u.ID, projectID: p.ID}
		}

		activeUser := addUserAndData("active@test.test", false, false)
		pendingDeletionUser := addUserAndData("pending@test.test", true, false)
		pendingDeletionUser2 := addUserAndData("pending2@test.test", true, false)
		frozenUser := addUserAndData("frozen@test.test", true, true)

		// share the active user's project with the pending deletion user.
		// the chore must not delete projects that are merely shared with a
		// pending deletion user but owned by someone else.
		_, err := sat.DB.Console().ProjectMembers().Insert(ctx, pendingDeletionUser.userID, activeUser.projectID, console.RoleMember)
		require.NoError(t, err)

		// Verify that all users have objects uploaded
		objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 4*userObjectsCount)

		chore.Loop.TriggerWait()

		testObjectsLength := func(usr projectAndUser, expected int) {
			usrObjs := make([]metabase.Object, 0)
			objs, err := sat.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)

			for i, object := range objs {
				if usr.projectID == object.ProjectID {
					usrObjs = append(usrObjs, objs[i])
				}
			}

			require.Len(t, usrObjs, expected)
		}

		// verify that all users have objects after the first chore run,
		// even those marked for deletion because the buffer time has not yet elapsed.
		for _, user := range []projectAndUser{activeUser, pendingDeletionUser, frozenUser, pendingDeletionUser2} {
			testObjectsLength(user, userObjectsCount)
		}

		chore.TestSetNowFn(func() time.Time {
			// set the chore time to some time beyond the buffer time
			return time.Now().Add(sat.Config.PendingDeleteCleanup.User.BufferTime + time.Hour)
		})
		chore.Loop.TriggerWait()

		// Verify that all objects are pending for user marked for deletion
		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, (3*userObjectsCount)-userObjectsCount)

		testDeactivated := func(user projectAndUser) {
			u, err := usersDB.Get(ctx, user.userID)
			require.NoError(t, err)
			require.Equal(t, console.Deleted, u.Status)

			// list all projects for the user, they should be deactivated
			projects, err := sat.DB.Console().Projects().GetOwn(ctx, user.userID)
			require.NoError(t, err)
			for _, p := range projects {
				require.NotNil(t, p.Status)
				require.Equal(t, console.ProjectDisabled, *p.Status)
			}
		}

		testDeactivated(pendingDeletionUser)
		testObjectsLength(pendingDeletionUser, 0)
		testDeactivated(pendingDeletionUser2)
		testObjectsLength(pendingDeletionUser2, 0)

		// the active user's project was shared with the (now deleted) pending
		// deletion user. It is owned by the still-active user, so its data must
		// remain untouched and the project must stay active.
		testObjectsLength(activeUser, userObjectsCount)
		activeUserProjects, err := sat.DB.Console().Projects().GetOwn(ctx, activeUser.userID)
		require.NoError(t, err)
		require.NotEmpty(t, activeUserProjects)
		for _, p := range activeUserProjects {
			require.NotNil(t, p.Status)
			require.Equal(t, console.ProjectActive, *p.Status)
		}

		// test that pending deletion user who is frozen is not deleted
		testObjectsLength(frozenUser, userObjectsCount)
		u, err := usersDB.Get(ctx, frozenUser.userID)
		require.NoError(t, err)
		require.Equal(t, console.PendingDeletion, u.Status)

		// make active user and frozen user deletable by the chore
		pending := console.PendingDeletion
		err = usersDB.Update(ctx, activeUser.userID, console.UpdateUserRequest{Status: &pending})
		require.NoError(t, err)

		err = accFreezeDB.DeleteAllByUserID(ctx, frozenUser.userID)
		require.NoError(t, err)

		// test that deletion is successful when concurrent delete is enabled

		chore.TestSetDeleteConcurrency(2)

		userList := make([]projectAndUser, 0)
		// add some frozen users with more data
		for i := range 4 {
			userList = append(userList, addUserAndData(fmt.Sprintf("pending+%d@test.test", i), true, false))
		}

		chore.Loop.TriggerWait()

		// no objects should be left
		// Verify that all objects are pending for users marked for deletion
		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Empty(t, objects)

		for _, user := range userList {
			testObjectsLength(user, 0)
			testDeactivated(user)
		}

		t.Run("nil status_updated_at user gets deleted", func(t *testing.T) {
			chore.TestSetNowFn(time.Now)

			nilUser := addUserAndData("nilstatus@test.test", true, false)

			// Simulate a legacy row: PendingDeletion with no status_updated_at.
			_, err = sat.DB.Testing().RawDB().ExecContext(ctx,
				sat.DB.Testing().Rebind("UPDATE users SET status_updated_at = NULL WHERE id = ?"),
				nilUser.userID,
			)
			require.NoError(t, err)

			u, err := usersDB.Get(ctx, nilUser.userID)
			require.NoError(t, err)
			require.Nil(t, u.StatusUpdatedAt)

			chore.Loop.TriggerWait()

			testObjectsLength(nilUser, 0)
			testDeactivated(nilUser)
		})
	})
}

// TestPendingDeleteChore_ObjectLockThresholds verifies the two deletion thresholds:
// buckets without object lock are deleted after BufferTime, while buckets with
// object lock enabled are retained until the later LockBufferTime and only then
// deleted (object lock is intentionally overridden for terminated accounts).
// The owning project/user is finalized only once nothing remains retained.
// Checked for both the project deletion path and the user deletion path.
func TestPendingDeleteChore_ObjectLockThresholds(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.PendingDeleteCleanup.Enabled = true
				config.PendingDeleteCleanup.Project.Enabled = true
				config.PendingDeleteCleanup.Project.BufferTime = time.Hour
				config.PendingDeleteCleanup.Project.LockBufferTime = 3 * time.Hour
				config.PendingDeleteCleanup.User.Enabled = true
				config.PendingDeleteCleanup.User.BufferTime = time.Hour
				config.PendingDeleteCleanup.User.LockBufferTime = 3 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]
		chore := sat.Core.ConsoleDBCleanup.PendingDeleteChore
		projectsDB := sat.DB.Console().Projects()
		usersDB := sat.DB.Console().Users()

		chore.Loop.Pause()

		// delete the default project to start fresh.
		require.NoError(t, projectsDB.Delete(ctx, upl.Projects[0].ID))

		now := time.Now().Truncate(time.Minute)
		projectsDB.TestSetNowFn(func() time.Time { return now })
		chore.TestSetNowFn(func() time.Time { return now })

		const lockedBucket = "locked-bucket"
		const plainBucket = "plain-bucket"

		// setupProject creates a project owned by ownerID holding two buckets, one
		// with object lock enabled and one without, each containing a single object.
		setupProject := func(ownerID uuid.UUID) uuid.UUID {
			p, err := sat.AddProject(ctx, ownerID, "object-lock-project")
			require.NoError(t, err)

			ownerCtx, err := sat.UserContext(ctx, ownerID)
			require.NoError(t, err)
			_, apiKey, err := sat.API.Console.Service.CreateAPIKey(
				ownerCtx, p.ID, "root", macaroon.APIKeyVersionObjectLock,
			)
			require.NoError(t, err)
			access, err := upl.Config.RequestAccessWithPassphrase(ctx, sat.URL(), apiKey.Serialize(), "")
			require.NoError(t, err)
			uplProject, err := uplink.OpenProject(ctx, access)
			require.NoError(t, err)
			defer ctx.Check(uplProject.Close)

			_, err = uplProject.EnsureBucket(ctx, lockedBucket)
			require.NoError(t, err)
			_, err = sat.DB.Buckets().UpdateBucketObjectLockSettings(ctx, buckets.UpdateBucketObjectLockParams{
				ObjectLockEnabled: true,
				ProjectID:         p.ID,
				Name:              lockedBucket,
			})
			require.NoError(t, err)

			_, err = uplProject.EnsureBucket(ctx, plainBucket)
			require.NoError(t, err)

			for _, bucket := range []string{lockedBucket, plainBucket} {
				upload, err := uplProject.UploadObject(ctx, bucket, "test-object", nil)
				require.NoError(t, err)
				_, err = upload.Write(testrand.Bytes(14 * memory.KiB))
				require.NoError(t, err)
				require.NoError(t, upload.Commit())
			}

			return p.ID
		}

		countObjects := func(projID uuid.UUID, bucket string) (count int) {
			objs, err := sat.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, o := range objs {
				if o.ProjectID == projID && o.BucketName == metabase.BucketName(bucket) {
					count++
				}
			}
			return count
		}

		owner, err := usersDB.GetByEmailAndTenant(ctx, upl.User[sat.ID()].Email, nil)
		require.NoError(t, err)

		// project deletion path: a project marked pending deletion.
		projectPathID := setupProject(owner.ID)
		require.NoError(t, projectsDB.UpdateStatus(ctx, projectPathID, console.ProjectPendingDeletion))

		// user deletion path: a user marked pending deletion, owning a project.
		pendingUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "pending_lock_user",
			Email:    "pending-lock@test.test",
		}, 1)
		require.NoError(t, err)
		userPathID := setupProject(pendingUser.ID)
		pd := console.PendingDeletion
		require.NoError(t, usersDB.Update(ctx, pendingUser.ID, console.UpdateUserRequest{Status: &pd}))

		// both buckets in both projects start with one object.
		for _, projID := range []uuid.UUID{projectPathID, userPathID} {
			require.Equal(t, 1, countObjects(projID, lockedBucket))
			require.Equal(t, 1, countObjects(projID, plainBucket))
		}

		// move past the buffer time and run the chore.
		chore.TestSetNowFn(func() time.Time { return now.Add(2 * time.Hour) })
		chore.Loop.TriggerWait()

		// project path: plain bucket data deleted, object lock data retained,
		// project kept pending deletion.
		require.Equal(t, 0, countObjects(projectPathID, plainBucket))
		require.Equal(t, 1, countObjects(projectPathID, lockedBucket))
		p, err := projectsDB.Get(ctx, projectPathID)
		require.NoError(t, err)
		require.NotNil(t, p.Status)
		require.Equal(t, console.ProjectPendingDeletion, *p.Status)

		// user path: plain bucket data deleted, object lock data retained, and
		// the user is not deactivated while object lock data remains.
		require.Equal(t, 0, countObjects(userPathID, plainBucket))
		require.Equal(t, 1, countObjects(userPathID, lockedBucket))
		u, err := usersDB.Get(ctx, pendingUser.ID)
		require.NoError(t, err)
		require.Equal(t, console.PendingDeletion, u.Status)
		up, err := projectsDB.Get(ctx, userPathID)
		require.NoError(t, err)
		require.NotNil(t, up.Status)
		require.Equal(t, console.ProjectActive, *up.Status)

		// move past the object lock buffer time and run again: object lock data
		// is now deleted (override) and the project/user are finalized.
		chore.TestSetNowFn(func() time.Time { return now.Add(4 * time.Hour) })
		chore.Loop.TriggerWait()

		// project path: object lock data deleted, project disabled.
		require.Equal(t, 0, countObjects(projectPathID, lockedBucket))
		p, err = projectsDB.Get(ctx, projectPathID)
		require.NoError(t, err)
		require.NotNil(t, p.Status)
		require.Equal(t, console.ProjectDisabled, *p.Status)

		// user path: object lock data deleted, project disabled, user deactivated.
		require.Equal(t, 0, countObjects(userPathID, lockedBucket))
		up, err = projectsDB.Get(ctx, userPathID)
		require.NoError(t, err)
		require.NotNil(t, up.Status)
		require.Equal(t, console.ProjectDisabled, *up.Status)
		u, err = usersDB.Get(ctx, pendingUser.ID)
		require.NoError(t, err)
		require.Equal(t, console.Deleted, u.Status)
	})
}

// TestPendingDeleteChore_Pagination verifies that the chore pages past retained
// (object lock) resources to reach deletable ones behind them, even when the
// number of retained resources exceeds ListLimit. The retained projects are the
// oldest, so they sit at the front of the (status_updated_at ASC) queue; a
// broken pagination (re-querying from offset 0, or advancing by batch size)
// would either loop forever on them or skip the deletable projects behind them.
func TestPendingDeleteChore_Pagination(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.PendingDeleteCleanup.Enabled = true
				config.PendingDeleteCleanup.Project.Enabled = true
				config.PendingDeleteCleanup.Project.BufferTime = 0
				config.PendingDeleteCleanup.Project.LockBufferTime = 720 * time.Hour
				config.PendingDeleteCleanup.ListLimit = 1 // force many single-item batches
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]
		chore := sat.Core.ConsoleDBCleanup.PendingDeleteChore
		projectsDB := sat.DB.Console().Projects()
		usersDB := sat.DB.Console().Users()

		chore.Loop.Pause()

		require.NoError(t, projectsDB.Delete(ctx, upl.Projects[0].ID))

		owner, err := usersDB.GetByEmailAndTenant(ctx, upl.User[sat.ID()].Email, nil)
		require.NoError(t, err)
		ownerCtx, err := sat.UserContext(ctx, owner.ID)
		require.NoError(t, err)

		now := time.Now().Truncate(time.Minute)

		// createProject marks a project pending deletion at markedAt, holding one
		// object in a bucket that is object-lock enabled (retained) or not (deletable).
		createProject := func(markedAt time.Time, objectLock bool) uuid.UUID {
			p, err := sat.AddProject(ctx, owner.ID, "p")
			require.NoError(t, err)

			_, apiKey, err := sat.API.Console.Service.CreateAPIKey(ownerCtx, p.ID, "root", macaroon.APIKeyVersionObjectLock)
			require.NoError(t, err)
			access, err := upl.Config.RequestAccessWithPassphrase(ctx, sat.URL(), apiKey.Serialize(), "")
			require.NoError(t, err)
			uplProject, err := uplink.OpenProject(ctx, access)
			require.NoError(t, err)
			defer ctx.Check(uplProject.Close)

			_, err = uplProject.EnsureBucket(ctx, "bucket")
			require.NoError(t, err)
			if objectLock {
				_, err = sat.DB.Buckets().UpdateBucketObjectLockSettings(ctx, buckets.UpdateBucketObjectLockParams{
					ObjectLockEnabled: true,
					ProjectID:         p.ID,
					Name:              "bucket",
				})
				require.NoError(t, err)
			}
			upload, err := uplProject.UploadObject(ctx, "bucket", "test-object", nil)
			require.NoError(t, err)
			_, err = upload.Write(testrand.Bytes(14 * memory.KiB))
			require.NoError(t, err)
			require.NoError(t, upload.Commit())

			// mark pending deletion with a controlled status_updated_at.
			projectsDB.TestSetNowFn(func() time.Time { return markedAt })
			require.NoError(t, projectsDB.UpdateStatus(ctx, p.ID, console.ProjectPendingDeletion))
			return p.ID
		}

		// The object lock (retained) projects are the oldest, so they head the
		// queue; there are more of them than ListLimit. The deletable projects are
		// newer and sit behind them.
		const retainedCount = 3
		const deletableCount = 3
		var retained, deletable []uuid.UUID
		for i := range retainedCount {
			retained = append(retained, createProject(now.Add(time.Duration(i)*time.Minute), true))
		}
		for i := range deletableCount {
			deletable = append(deletable, createProject(now.Add(time.Duration(retainedCount+i)*time.Minute), false))
		}

		countObjects := func(projID uuid.UUID) (count int) {
			objs, err := sat.Metabase.DB.TestingAllObjects(ctx)
			require.NoError(t, err)
			for _, o := range objs {
				if o.ProjectID == projID {
					count++
				}
			}
			return count
		}

		// run past BufferTime (0) but well before LockBufferTime (720h).
		chore.TestSetNowFn(func() time.Time { return now.Add(time.Hour) })
		chore.Loop.TriggerWait()

		// every deletable project behind the retained ones was reached: its data
		// is gone and the project is disabled.
		for _, id := range deletable {
			require.Equal(t, 0, countObjects(id), "deletable project should have no objects")
			p, err := projectsDB.Get(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, p.Status)
			require.Equal(t, console.ProjectDisabled, *p.Status)
		}

		// the object lock projects were retained: still pending, data intact.
		for _, id := range retained {
			require.Equal(t, 1, countObjects(id), "object lock project should retain its object")
			p, err := projectsDB.Get(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, p.Status)
			require.Equal(t, console.ProjectPendingDeletion, *p.Status)
		}
	})
}

func TestPendingDeleteChore_MinimumRetentionCharges(t *testing.T) {
	// Configure 30 day minimum retention (720 hours)
	minRetentionProduct := paymentsconfig.ProductUsagePrice{
		ID:   1,
		Name: "Min Retention Product",
		ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{
			StorageTB: "4",
			EgressTB:  "7",
			Segment:   "0.0000088",
		},
		MinimumRetentionDuration: "720h",
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.PendingDeleteCleanup.Enabled = true
				config.PendingDeleteCleanup.Project.Enabled = true
				config.PendingDeleteCleanup.Project.BufferTime = time.Hour
				config.Metainfo.CreateRemainderChargeOnObjectDelete = true
				config.Payments.Products.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
					minRetentionProduct.ID: minRetentionProduct,
				})
				config.Payments.PlacementPriceOverrides.SetMap(map[int]int32{
					int(storj.DefaultPlacement): minRetentionProduct.ID,
				})
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]
		project := upl.Projects[0]
		chore := sat.Core.ConsoleDBCleanup.PendingDeleteChore
		projectsDB := sat.DB.Console().Projects()

		chore.Loop.Pause()

		now := time.Now()
		now = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		projectsDB.TestSetNowFn(func() time.Time { return now })
		chore.TestSetNowFn(func() time.Time { return now })

		bucketName := testrand.BucketName()
		require.NoError(t, upl.CreateBucket(ctx, sat, bucketName))
		err := upl.Upload(ctx, sat, bucketName, "test-object", testrand.Bytes(10*memory.KiB))
		require.NoError(t, err)

		// Verify the object was uploaded
		objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 1)

		obj := objects[0]

		// Set object creation time to 20 days ago (480 hours) - within the 30-day retention period
		_, err = sat.Metabase.DB.TestingSetObjectCreatedAt(ctx, obj.ObjectStream, now.Add(-480*time.Hour))
		require.NoError(t, err)

		// Mark the project for deletion
		err = projectsDB.UpdateStatus(ctx, project.ID, console.ProjectPendingDeletion)
		require.NoError(t, err)

		// The chore must run after the buffer time elapses
		choreTime := now.Add(sat.Config.PendingDeleteCleanup.Project.BufferTime + time.Hour)
		chore.TestSetNowFn(func() time.Time { return choreTime })
		chore.Loop.TriggerWait()

		// Verify that objects are deleted
		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Empty(t, objects, "objects should be deleted after buffer time elapses")

		// Verify that a retention remainder charge was created
		chargeOptions := accounting.GetUnbilledChargesOptions{
			ProjectID: project.ID,
			From:      now.Add(-24 * time.Hour),
			To:        now.Add(48 * time.Hour),
			Limit:     10,
		}

		charges, _, err := sat.DB.RetentionRemainderCharges().GetUnbilledCharges(ctx, chargeOptions)
		require.NoError(t, err)
		require.Len(t, charges, 1, "expected exactly one retention remainder charge")

		charge := charges[0]
		require.Equal(t, project.ID, charge.ProjectID)
		require.Equal(t, minRetentionProduct.ID, charge.ProductID)
		require.Equal(t, bucketName, charge.BucketName)
		require.False(t, charge.Billed)

		storageHours := choreTime.Sub(now.Add(-480 * time.Hour)).Hours()
		remainingHours := 720 - storageHours
		require.InDelta(t, remainingHours*float64(obj.TotalEncryptedSize), charge.RemainderByteHours, 1.0,
			"expected remaining byte-hours to be approximately %.0f hours * object size", remainingHours)

		// Verify the project is disabled
		p, err := projectsDB.Get(ctx, project.ID)
		require.NoError(t, err)
		require.NotNil(t, p.Status)
		require.Equal(t, console.ProjectDisabled, *p.Status)
	})
}
