// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter_test

import (
	"archive/zip"
	"bytes"
	"io/ioutil"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/internalpb"
)

func TestGarbageCollectionBloomFilters(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 7,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.SegmentLoop.AsOfSystemInterval = 1

				testplanet.ReconfigureRS(2, 2, 7, 7)(log, index, config)
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object", testrand.Bytes(10*memory.KiB))
		require.NoError(t, err)

		access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
		accessString, err := access.Serialize()
		require.NoError(t, err)

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		type testCase struct {
			Bucket        string
			ZipBatchSize  int
			ExpectedPacks int
		}

		testCases := []testCase{
			{"bloomfilters-bucket-1", 1, 7},
			{"bloomfilters-bucket-2", 2, 4},
			{"bloomfilters-bucket-7", 7, 1},
			{"bloomfilters-bucket-100", 100, 1},
		}

		for _, tc := range testCases {
			config := planet.Satellites[0].Config.GarbageCollectionBF
			config.Enabled = true
			config.AccessGrant = accessString
			config.Bucket = tc.Bucket
			config.ZipBatchSize = tc.ZipBatchSize
			service := bloomfilter.NewService(zaptest.NewLogger(t), config, planet.Satellites[0].Overlay.DB, planet.Satellites[0].Metabase.SegmentLoop)

			err = service.RunOnce(ctx)
			require.NoError(t, err)

			iterator := project.ListObjects(ctx, tc.Bucket, nil)

			count := 0
			nodeIds := []string{}
			packNames := []string{}
			for iterator.Next() {
				packNames = append(packNames, iterator.Item().Key)

				data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], tc.Bucket, iterator.Item().Key)
				require.NoError(t, err)

				zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
				require.NoError(t, err)

				for _, file := range zipReader.File {
					bfReader, err := file.Open()
					require.NoError(t, err)

					bloomfilter, err := ioutil.ReadAll(bfReader)
					require.NoError(t, err)

					var pbRetainInfo internalpb.RetainInfo
					err = pb.Unmarshal(bloomfilter, &pbRetainInfo)
					require.NoError(t, err)

					require.NotEmpty(t, pbRetainInfo.Filter)
					require.NotZero(t, pbRetainInfo.PieceCount)
					require.NotZero(t, pbRetainInfo.CreationDate)
					require.Equal(t, file.Name, pbRetainInfo.StorageNodeId.String())

					nodeIds = append(nodeIds, pbRetainInfo.StorageNodeId.String())
				}

				count++
			}
			require.NoError(t, iterator.Err())
			require.Equal(t, tc.ExpectedPacks, count)

			expectedPackNames := []string{}
			for i := 0; i < tc.ExpectedPacks; i++ {
				expectedPackNames = append(expectedPackNames, "bloomfilters-"+strconv.Itoa(i)+".zip")
			}
			sort.Strings(expectedPackNames)
			sort.Strings(packNames)
			require.Equal(t, expectedPackNames, packNames)

			expectedNodeIds := []string{}
			for _, node := range planet.StorageNodes {
				expectedNodeIds = append(expectedNodeIds, node.ID().String())
			}
			sort.Strings(expectedNodeIds)
			sort.Strings(nodeIds)
			require.Equal(t, expectedNodeIds, nodeIds)
		}
	})
}

func TestGarbageCollectionBloomFilters_NotEmptyBucket(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.SegmentLoop.AsOfSystemInterval = 1

				testplanet.ReconfigureRS(2, 2, 4, 4)(log, index, config)
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object", testrand.Bytes(10*memory.KiB))
		require.NoError(t, err)

		access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
		accessString, err := access.Serialize()
		require.NoError(t, err)

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bloomfilters", "some object", testrand.Bytes(1*memory.KiB))
		require.NoError(t, err)

		config := planet.Satellites[0].Config.GarbageCollectionBF
		config.AccessGrant = accessString
		config.Bucket = "bloomfilters"
		service := bloomfilter.NewService(zaptest.NewLogger(t), config, planet.Satellites[0].Overlay.DB, planet.Satellites[0].Metabase.SegmentLoop)

		err = service.RunOnce(ctx)
		require.NoError(t, err)

		iterator := project.ListObjects(ctx, "bloomfilters", nil)

		// no new uploads, bucket structure was not changed
		require.True(t, iterator.Next())
		require.Equal(t, "some object", iterator.Item().Key)
		require.False(t, iterator.Next())
		require.NoError(t, iterator.Err())
	})
}
