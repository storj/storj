// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package expireddeletion_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/storage"
)

// TestExpiredDeletion does the following:
// * Set up a network with one storagenode
// * Upload three segments
// * Run the expired segment chore
// * Verify that all three segments still exist
// * Expire one of the segments
// * Run the expired segment chore
// * Verify that two segments still exist and the expired one has been deleted
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

		// Upload three objects
		for i := 0; i < 4; i++ {
			// 0 and 1 inline, 2 and 3 remote
			testData := testrand.Bytes(8 * memory.KiB)
			if i <= 1 {
				testData = testrand.Bytes(1 * memory.KiB)

			}
			err := upl.Upload(ctx, satellite, "testbucket", "test/path/"+strconv.Itoa(i), testData)
			require.NoError(t, err)
		}

		// Wait for next iteration of expired cleanup to finish
		expiredChore.Loop.Restart()
		expiredChore.Loop.TriggerWait()

		// Verify that all four objects exist in metainfo
		var expiredInline storage.ListItem
		var expiredRemote storage.ListItem
		i := 0
		err := satellite.Metainfo.Database.Iterate(ctx, storage.IterateOptions{Recurse: true},
			func(ctx context.Context, it storage.Iterator) error {
				var item storage.ListItem
				for it.Next(ctx, &item) {
					if i == 1 {
						expiredInline = item
					} else if i == 3 {
						expiredRemote = item
					}
					i++
				}
				return nil
			})
		require.NoError(t, err)
		require.EqualValues(t, i, 4)

		// Expire one inline segment and one remote segment
		pointer := &pb.Pointer{}
		err = pb.Unmarshal(expiredInline.Value, pointer)
		require.NoError(t, err)
		pointer.ExpirationDate = time.Now().Add(-24 * time.Hour)
		newPointerBytes, err := pb.Marshal(pointer)
		require.NoError(t, err)
		err = satellite.Metainfo.Database.CompareAndSwap(ctx, expiredInline.Key, expiredInline.Value, newPointerBytes)
		require.NoError(t, err)

		err = pb.Unmarshal(expiredRemote.Value, pointer)
		require.NoError(t, err)
		pointer.ExpirationDate = time.Now().Add(-24 * time.Hour)
		newPointerBytes, err = pb.Marshal(pointer)
		require.NoError(t, err)
		err = satellite.Metainfo.Database.CompareAndSwap(ctx, expiredRemote.Key, expiredRemote.Value, newPointerBytes)
		require.NoError(t, err)

		// Wait for next iteration of expired cleanup to finish
		expiredChore.Loop.Restart()
		expiredChore.Loop.TriggerWait()

		// Verify that only two objects exist in metainfo
		// And that the expired ones do not exist
		i = 0
		err = satellite.Metainfo.Database.Iterate(ctx, storage.IterateOptions{Recurse: true},
			func(ctx context.Context, it storage.Iterator) error {
				var item storage.ListItem
				for it.Next(ctx, &item) {
					require.False(t, item.Key.Equal(expiredInline.Key))
					require.False(t, item.Key.Equal(expiredRemote.Key))
					i++
				}
				return nil
			})
		require.NoError(t, err)
		require.EqualValues(t, i, 2)
	})
}
