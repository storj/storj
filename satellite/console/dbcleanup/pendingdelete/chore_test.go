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
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
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
			// set the chore time to some time beyond the escalation buffer time
			return time.Now().Add(sat.Config.PendingDeleteCleanup.BillingFreeze.BufferTime + time.Hour)
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
	})
}
