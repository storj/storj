// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metainfo/metabase"
)

const lastSegmentIndex = -1

// TestGetItems_ReturnValueOrder ensures the return value
// of GetItems will always be the same order as the requested paths.
// The test does following steps:
// - Uploads test data (multi-segment objects)
// - Gather all object paths with an extra invalid path at random position
// - Retrieve pointers using above paths
// - Ensure the nil pointer and last segment paths are in the same order as their
// corresponding paths.
func TestGetItems_ReturnValueOrder(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 4, 4),
				testplanet.MaxSegmentSize(3*memory.KiB),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		satellite := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		numItems := 5
		for i := 0; i < numItems; i++ {
			path := fmt.Sprintf("test/path_%d", i)
			err := uplinkPeer.Upload(ctx, satellite, "bucket", path, testrand.Bytes(15*memory.KiB))
			require.NoError(t, err)
		}

		keys, err := satellite.Metainfo.Database.List(ctx, nil, numItems)
		require.NoError(t, err)

		var segmentKeys = make([]metabase.SegmentKey, 0, numItems+1)
		var lastSegmentPathIndices []int

		// Random nil pointer
		nilPointerIndex := testrand.Intn(numItems + 1)

		for i, key := range keys {
			segmentKeys = append(segmentKeys, metabase.SegmentKey(key))
			segmentIdx, err := parseSegmentPath([]byte(key.String()))
			require.NoError(t, err)

			if segmentIdx == lastSegmentIndex {
				lastSegmentPathIndices = append(lastSegmentPathIndices, i)
			}

			// set a random path to be nil.
			if nilPointerIndex == i {
				segmentKeys[nilPointerIndex] = nil
			}
		}

		pointers, err := satellite.Metainfo.Service.GetItems(ctx, segmentKeys)
		require.NoError(t, err)

		for i, p := range pointers {
			if p == nil {
				require.Equal(t, nilPointerIndex, i)
				continue
			}

			meta := pb.StreamMeta{}
			metaInBytes := p.GetMetadata()
			err = pb.Unmarshal(metaInBytes, &meta)
			require.NoError(t, err)

			lastSegmentMeta := meta.GetLastSegmentMeta()
			if lastSegmentMeta != nil {
				require.Equal(t, lastSegmentPathIndices[i], i)
			}
		}
	})
}

func TestCountBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saPeer := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]
		projectID := planet.Uplinks[0].Projects[0].ID
		count, err := saPeer.Metainfo.Service.CountBuckets(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, 0, count)
		// Setup: create 2 test buckets
		err = uplinkPeer.CreateBucket(ctx, saPeer, "test1")
		require.NoError(t, err)
		count, err = saPeer.Metainfo.Service.CountBuckets(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		err = uplinkPeer.CreateBucket(ctx, saPeer, "test2")
		require.NoError(t, err)
		count, err = saPeer.Metainfo.Service.CountBuckets(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})
}

func parseSegmentPath(segmentPath []byte) (segmentIndex int64, err error) {
	elements := storj.SplitPath(string(segmentPath))
	if len(elements) < 4 {
		return -1, errs.New("invalid path %q", string(segmentPath))
	}

	// var segmentIndex int64
	if elements[1] == "l" {
		segmentIndex = lastSegmentIndex
	} else {
		segmentIndex, err = strconv.ParseInt(elements[1][1:], 10, 64)
		if err != nil {
			return lastSegmentIndex, errs.Wrap(err)
		}
	}
	return segmentIndex, nil
}

func TestIsBucketEmpty(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		err := uplinkPeer.CreateBucket(ctx, satellite, "bucket")
		require.NoError(t, err)

		empty, err := satellite.Metainfo.Service.IsBucketEmpty(ctx, uplinkPeer.Projects[0].ID, []byte("bucket"))
		require.NoError(t, err)
		require.True(t, empty)

		err = uplinkPeer.Upload(ctx, satellite, "bucket", "test/path", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		empty, err = satellite.Metainfo.Service.IsBucketEmpty(ctx, uplinkPeer.Projects[0].ID, []byte("bucket"))
		require.NoError(t, err)
		require.False(t, empty)
	})
}
