// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package zombiedeletion_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
)

func TestZombieDeletion(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.ZombieDeletion.Enabled = true
				config.ZombieDeletion.Interval = 500 * time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		upl := planet.Uplinks[0]
		zombieChore := planet.Satellites[0].Core.ZombieDeletion.Chore

		zombieChore.Loop.Pause()

		err := upl.CreateBucket(ctx, planet.Satellites[0], "testbucket1")
		require.NoError(t, err)

		err = upl.CreateBucket(ctx, planet.Satellites[0], "testbucket2")
		require.NoError(t, err)

		err = upl.CreateBucket(ctx, planet.Satellites[0], "testbucket3")
		require.NoError(t, err)

		// upload regular object, will be NOT deleted
		err = upl.Upload(ctx, planet.Satellites[0], "testbucket1", "committed_object", testrand.Bytes(1*memory.KiB))
		require.NoError(t, err)

		project, err := upl.OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		_, err = project.BeginUpload(ctx, "testbucket2", "zombie_object_multipart_no_segment", nil)
		require.NoError(t, err)

		// upload pending object with multipart upload, will be deleted
		info, err := project.BeginUpload(ctx, "testbucket3", "zombie_object_multipart", nil)
		require.NoError(t, err)
		partUpload, err := project.UploadPart(ctx, "testbucket3", "zombie_object_multipart", info.UploadID, 1)
		require.NoError(t, err)
		_, err = partUpload.Write(testrand.Bytes(1 * memory.KiB))
		require.NoError(t, err)
		err = partUpload.Commit()
		require.NoError(t, err)

		// Verify that all objects are in the metabase
		objects, err := planet.Satellites[0].Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 3)

		// Trigger the next iteration of cleanup and wait to finish
		zombieChore.TestingSetNow(func() time.Time {
			// Set the Now function to return time after the objects zombie deadline time
			return time.Now().Add(25 * time.Hour)
		})
		zombieChore.Loop.TriggerWait()

		// Verify that only one object remain in the metabase
		objects, err = planet.Satellites[0].Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 1)
		require.Equal(t, metabase.Committed, objects[0].Status)
	})
}
