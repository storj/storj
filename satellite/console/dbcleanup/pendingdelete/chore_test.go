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
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/uplink"
)

func TestProjectPendingDeleteChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.PendingDeleteCleanup.Enabled = true
				config.PendingDeleteCleanup.BufferTime = time.Hour
				config.PendingDeleteCleanup.ListLimit = 2 // small limit to test batching
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]
		chore := sat.Core.ConsoleDBCleanup.PendingDeleteChore
		projectsDB := sat.DB.Console().Projects()
		usersDB := sat.DB.Console().Users()

		chore.Loop.Pause()

		user, err := usersDB.GetByEmail(ctx, upl.User[sat.ID()].Email)
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

		type idAndUplink struct {
			upl       *uplink.Project
			projectID uuid.UUID
		}

		addProjectAndData := func(status console.ProjectStatus) idAndUplink {
			p, err := sat.AddProject(ctx, user.ID, "new project")
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

			for j := range objectsCount {
				upload, err := uplProject.UploadObject(ctx, "test-bucket", fmt.Sprintf("test-object-%d", j), nil)
				require.NoError(t, err)
				_, err = upload.Write(testrand.Bytes(14 * memory.KiB))
				require.NoError(t, err)
				require.NoError(t, upload.Commit())
			}

			if status != console.ProjectActive {
				err = projectsDB.UpdateStatus(ctx, p.ID, status)
				require.NoError(t, err)
			}

			return idAndUplink{upl: uplProject, projectID: p.ID}
		}

		projectsMarkedForDeletion := make([]idAndUplink, 0)
		activeProjects := make([]idAndUplink, 0)

		for i := range projectsCount {
			if i%2 == 0 {
				projectsMarkedForDeletion = append(projectsMarkedForDeletion, addProjectAndData(console.ProjectPendingDeletion))
				continue
			}
			activeProjects = append(activeProjects, addProjectAndData(console.ProjectActive))
		}

		// Verify that all four projects have objects uploaded
		objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, projectsCount*objectsCount)

		chore.Loop.TriggerWait()

		testObjectsLength := func(upl idAndUplink, expected int) {
			itr := upl.upl.ListObjects(ctx, "test-bucket", nil)
			count := 0
			for itr.Next() {
				count++
			}
			require.NoError(t, itr.Err())
			require.Equal(t, expected, count)
		}

		// verify that all users have objects after the first chore run,
		// even those marked for deletion because the buffer time has not yet elapsed.
		for _, project := range projectsMarkedForDeletion {
			// verify that the user has objects
			testObjectsLength(project, objectsCount)
		}
		for _, project := range activeProjects {
			// verify that the user has objects
			testObjectsLength(project, objectsCount)
		}

		chore.TestSetNowFn(func() time.Time {
			// set the chore time to some time beyond the buffer time
			return now.Add(sat.Config.PendingDeleteCleanup.BufferTime + (24 * time.Hour))
		})
		chore.Loop.TriggerWait()

		// Verify that all objects are deleted for projects marked for deletion
		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, (projectsCount*objectsCount)/2)

		testDisabled := func(upl idAndUplink) {
			p, err := projectsDB.Get(ctx, upl.projectID)
			require.NoError(t, err)
			require.NotNil(t, p.Status)
			require.Equal(t, console.ProjectDisabled, *p.Status)
		}

		for _, project := range projectsMarkedForDeletion {
			// verify that marked projects have no objects and
			// are disabled.
			testObjectsLength(project, 0)
			testDisabled(project)
		}
		for _, project := range activeProjects {
			// verify that the user has objects
			testObjectsLength(project, objectsCount)
		}

		// test that deletion is successful when concurrent delete is enabled
		chore.TestSetDeleteConcurrency(2)

		newProjectsList := make([]idAndUplink, 0)
		for range projectsCount {
			newProjectsList = append(newProjectsList, addProjectAndData(console.ProjectPendingDeletion))
		}

		// mark active projects for deletion
		for i, p := range activeProjects {
			projectsDB.TestSetNowFn(func() time.Time { return now.Add(time.Duration(i) * time.Minute) })
			err = projectsDB.UpdateStatus(ctx, p.projectID, console.ProjectPendingDeletion)
			require.NoError(t, err)
		}

		newProjectsList = append(newProjectsList, activeProjects...)

		chore.Loop.TriggerWait()

		// Verify that all objects are deleted for projects marked for deletion
		objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Empty(t, objects)

		for _, p := range newProjectsList {
			testObjectsLength(p, 0)
			testDisabled(p)
		}
	})
}
