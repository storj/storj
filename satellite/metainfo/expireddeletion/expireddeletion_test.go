// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package expireddeletion_test

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
		objects, err := satellite.Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 4)

		// Trigger the next iteration of expired cleanup and wait to finish
		expiredChore.SetNow(func() time.Time {
			// Set the Now function to return time after the objects expiration time
			return time.Now().Add(2 * time.Hour)
		})
		expiredChore.Loop.TriggerWait()

		// Verify that only two objects remain in the metabase
		objects, err = satellite.Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 2)
	})
}
