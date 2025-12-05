// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package expireddeletion_test

import (
	"strconv"
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

func TestExpiredDeletion(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.ExpiredDeletion.Interval = 500 * time.Millisecond
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		upl := planet.Uplinks[0]
		expiredChore := satellite.Core.ExpiredDeletion.Chore

		expiredChore.Loop.Pause()

		// Upload four objects:
		// 1. Inline object without expiraton date
		err := upl.Upload(ctx, satellite, "testbucket", "inline_no_expire", testrand.Bytes(1*memory.KiB))
		require.NoError(t, err)
		// 2. Remote object without expiraton date
		err = upl.Upload(ctx, satellite, "testbucket", "remote_no_expire", testrand.Bytes(8*memory.KiB))
		require.NoError(t, err)
		// 3. Inline object with expiraton date
		err = upl.UploadWithExpiration(ctx, satellite, "testbucket", "inline_expire", testrand.Bytes(1*memory.KiB), time.Now().Add(1*time.Hour))
		require.NoError(t, err)
		// 4. Remote object with expiraton date
		err = upl.UploadWithExpiration(ctx, satellite, "testbucket", "remote_expire", testrand.Bytes(8*memory.KiB), time.Now().Add(1*time.Hour))
		require.NoError(t, err)

		// Verify that all four objects are in the metabase
		objects, err := satellite.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 4)

		// Trigger the next iteration of expired cleanup and wait to finish
		expiredChore.SetNow(func() time.Time {
			// Set the Now function to return time after the objects expiration time
			return time.Now().Add(2 * time.Hour)
		})
		expiredChore.Loop.TriggerWait()

		// Verify that only two objects remain in the metabase
		objects, err = satellite.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 2)
	})
}

func TestExpiresAtForSegmentsAfterCopy(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		upl := planet.Uplinks[0]

		expiredChore := satellite.Core.ExpiredDeletion.Chore
		expiredChore.Loop.Pause()

		now := time.Now()
		expiresAt := now.Add(1 * time.Hour)

		// upload inline object with expiration time
		err := upl.UploadWithExpiration(ctx, satellite, "testbucket", "inline_expire", testrand.Bytes(memory.KiB), expiresAt)
		require.NoError(t, err)

		// upload remote object with expiration time
		err = upl.UploadWithExpiration(ctx, satellite, "testbucket", "remote_expire", testrand.Bytes(8*memory.KiB), expiresAt)
		require.NoError(t, err)

		project, err := upl.GetProject(ctx, satellite)
		require.NoError(t, err)
		ctx.Check(project.Close)

		// copy inline object
		_, err = project.CopyObject(ctx, "testbucket", "inline_expire", "testbucket", "new_inline_expire", nil)
		require.NoError(t, err)

		// copy remote object
		_, err = project.CopyObject(ctx, "testbucket", "remote_expire", "testbucket", "new_remote_expire", nil)
		require.NoError(t, err)
		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		objects, err := satellite.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 4)

		for _, v := range objects {
			result, err := satellite.Metabase.DB.ListSegments(ctx, metabase.ListSegments{
				StreamID: v.StreamID,
			})
			require.NoError(t, err)

			for _, k := range result.Segments {
				require.Equal(t, expiresAt.Unix(), k.ExpiresAt.Unix())
			}
		}
	})
}

func TestExpiredDeletionForCopiedObject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		upl := planet.Uplinks[0]
		expiredChore := satellite.Core.ExpiredDeletion.Chore

		expiredChore.Loop.Pause()

		// upload inline object with expiration time
		err := upl.UploadWithExpiration(ctx, satellite, "testbucket", "inline_expire", testrand.Bytes(memory.KiB), time.Now().Add(1*time.Hour))
		require.NoError(t, err)

		// upload remote object with expiration time
		err = upl.UploadWithExpiration(ctx, satellite, "testbucket", "remote_expire", testrand.Bytes(8*memory.KiB), time.Now().Add(1*time.Hour))
		require.NoError(t, err)

		project, err := upl.GetProject(ctx, satellite)
		require.NoError(t, err)
		ctx.Check(project.Close)

		// copy inline object
		_, err = project.CopyObject(ctx, "testbucket", "inline_expire", "testbucket", "new_inline_expire", nil)
		require.NoError(t, err)

		// copy remote object
		_, err = project.CopyObject(ctx, "testbucket", "remote_expire", "testbucket", "new_remote_expire", nil)
		require.NoError(t, err)

		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

		objects, err := satellite.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 4)

		// Trigger the next iteration of expired cleanup and wait to finish
		expiredChore.SetNow(func() time.Time {
			// Set the Now function to return time after the objects expiration time
			return time.Now().Add(2 * time.Hour)
		})
		expiredChore.Loop.TriggerWait()

		// Verify that 0 objects remain in the metabase
		objects, err = satellite.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 0)
	})
}

func TestExpiredDeletion_ConcurrentDeletes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.ExpiredDeletion.DeleteConcurrency = 3
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		upl := planet.Uplinks[0]
		expiredChore := satellite.Core.ExpiredDeletion.Chore
		expiredChore.Loop.Pause()

		expiredChore.Loop.Pause()

		for i := 0; i < 31; i++ {
			err := upl.UploadWithExpiration(ctx, satellite, "testbucket", "inline_no_expire"+strconv.Itoa(i), testrand.Bytes(1*memory.KiB), time.Now().Add(1*time.Hour))
			require.NoError(t, err)
		}

		// Verify that all objects are in the metabase
		objects, err := satellite.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 31)

		// Trigger the next iteration of expired cleanup and wait to finish
		expiredChore.SetNow(func() time.Time {
			// Set the Now function to return time after the objects expiration time
			return time.Now().Add(2 * time.Hour)
		})
		expiredChore.Loop.TriggerWait()

		// Verify that all objects are deleted from the metabase
		objects, err = satellite.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 0)
	})
}
