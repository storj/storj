// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/exp/slices"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/rangedloop/rangedlooptest"
	"storj.io/storj/satellite/overlay"
	"storj.io/uplink"
)

func TestObserverGarbageCollectionBloomFilters(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 7,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 2, 7, 7),
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
			Bucket            string
			ZipBatchSize      int
			ExpectedPacks     int
			DisqualifiedNodes []storj.NodeID
		}

		testCases := []testCase{
			{"bloomfilters-bucket-1", 1, 7, []storj.NodeID{}},
			{"bloomfilters-bucket-2", 2, 4, []storj.NodeID{}},
			{"bloomfilters-bucket-2-dq-nodes", 2, 3, []storj.NodeID{
				planet.StorageNodes[0].ID(),
				planet.StorageNodes[3].ID(),
			}},
			{"bloomfilters-bucket-7", 7, 1, []storj.NodeID{}},
			{"bloomfilters-bucket-100", 100, 1, []storj.NodeID{}},
		}

		for _, tc := range testCases {
			// TODO: this is a little chicken-and-eggy... the GCBF config is
			// provided to the rangedloop service above, but we don't have the
			// access grant available until after testplanet has configured
			// everything. For now, just test the bloomfilter observer
			// directly, as is already done for the service. Maybe we can
			// improve this later.
			config := planet.Satellites[0].Config.GarbageCollectionBF
			config.AccessGrant = accessString
			config.Bucket = tc.Bucket
			config.ZipBatchSize = tc.ZipBatchSize
			observers := []rangedloop.Observer{
				bloomfilter.NewObserver(zaptest.NewLogger(t), config, planet.Satellites[0].Overlay.DB),
				bloomfilter.NewSyncObserver(zaptest.NewLogger(t), config, planet.Satellites[0].Overlay.DB),
			}

			expectedNodeIds := []string{}
			for _, node := range planet.StorageNodes {
				db := planet.Satellites[0].DB.Testing()
				_, err := db.RawDB().ExecContext(ctx, db.Rebind("UPDATE nodes SET disqualified = null WHERE id = ?"), node.ID())
				require.NoError(t, err)

				expectedNodeIds = append(expectedNodeIds, node.ID().String())
			}

			for _, nodeID := range tc.DisqualifiedNodes {
				require.NoError(t, planet.Satellites[0].Overlay.Service.DisqualifyNode(ctx, nodeID, overlay.DisqualificationReasonAuditFailure))

				if index := slices.Index(expectedNodeIds, nodeID.String()); index != -1 {
					expectedNodeIds = slices.Delete(expectedNodeIds, index, index+1)
				}
			}

			sort.Strings(expectedNodeIds)

			for _, observer := range observers {
				name := fmt.Sprintf("%s-%T", tc.Bucket, observer)
				t.Run(name, func(t *testing.T) {
					// TODO: see comment above. ideally this should use the rangedloop
					// service instantiated for the testplanet.
					rangedloopConfig := planet.Satellites[0].Config.RangedLoop
					segments := rangedloop.NewMetabaseRangeSplitter(zap.NewNop(), planet.Satellites[0].Metabase.DB, rangedloopConfig)
					rangedLoop := rangedloop.NewService(zap.NewNop(), planet.Satellites[0].Config.RangedLoop, segments,
						[]rangedloop.Observer{observer})

					_, err = rangedLoop.RunOnce(ctx)
					require.NoError(t, err)

					download, err := project.DownloadObject(ctx, tc.Bucket, bloomfilter.LATEST, nil)
					require.NoError(t, err)

					value, err := io.ReadAll(download)
					require.NoError(t, err)

					err = download.Close()
					require.NoError(t, err)

					prefix := string(value)
					iterator := project.ListObjects(ctx, tc.Bucket, &uplink.ListObjectsOptions{
						Prefix: prefix + "/",
					})

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

							bloomfilter, err := io.ReadAll(bfReader)
							require.NoError(t, err)

							var pbRetainInfo internalpb.RetainInfo
							err = pb.Unmarshal(bloomfilter, &pbRetainInfo)
							require.NoError(t, err)

							require.NotEmpty(t, pbRetainInfo.Filter)
							require.NotZero(t, pbRetainInfo.PieceCount)
							require.NotZero(t, pbRetainInfo.CreationDate)
							require.Equal(t, file.Name, pbRetainInfo.StorageNodeId.String())

							nodeIds = append(nodeIds, pbRetainInfo.StorageNodeId.String())

							nodeID, err := storj.NodeIDFromBytes(pbRetainInfo.StorageNodeId.Bytes())
							require.NoError(t, err)
							require.NotContains(t, tc.DisqualifiedNodes, nodeID)
						}

						count++
					}
					require.NoError(t, iterator.Err())
					require.Equal(t, tc.ExpectedPacks, count)

					expectedPackNames := []string{}
					for i := 0; i < tc.ExpectedPacks; i++ {
						expectedPackNames = append(expectedPackNames, prefix+"/bloomfilters-"+strconv.Itoa(i)+".zip")
					}
					sort.Strings(expectedPackNames)
					sort.Strings(packNames)
					require.Equal(t, expectedPackNames, packNames)

					sort.Strings(nodeIds)
					require.Equal(t, expectedNodeIds, nodeIds)
				})
			}
		}
	})
}

func TestObserverGarbageCollectionBloomFilters_AllowNotEmptyBucket(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
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

		// TODO: this is a little chicken-and-eggy... the GCBF config is
		// provided to the rangedloop service above, but we don't have the
		// access grant available until after testplanet has configured
		// everything. For now, just test the bloomfilter observer
		// directly, as is already done for the service. Maybe we can
		// improve this later.
		config := planet.Satellites[0].Config.GarbageCollectionBF
		config.AccessGrant = accessString
		config.Bucket = "bloomfilters"

		for _, observer := range []rangedloop.Observer{
			bloomfilter.NewObserver(zaptest.NewLogger(t), config, planet.Satellites[0].Overlay.DB),
			bloomfilter.NewSyncObserver(zaptest.NewLogger(t), config, planet.Satellites[0].Overlay.DB),
		} {
			t.Run(fmt.Sprintf("%T", observer), func(t *testing.T) {
				// TODO: see comment above. ideally this should use the rangedloop
				// service instantiated for the testplanet.
				rangedloopConfig := planet.Satellites[0].Config.RangedLoop
				segments := rangedloop.NewMetabaseRangeSplitter(zap.NewNop(), planet.Satellites[0].Metabase.DB, rangedloopConfig)
				rangedLoop := rangedloop.NewService(zap.NewNop(), planet.Satellites[0].Config.RangedLoop, segments,
					[]rangedloop.Observer{observer})

				_, err = rangedLoop.RunOnce(ctx)
				require.NoError(t, err)

				latestData, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], config.Bucket, bloomfilter.LATEST)
				require.NoError(t, err)

				iterator := project.ListObjects(ctx, config.Bucket, &uplink.ListObjectsOptions{
					Prefix: string(latestData) + "/",
				})
				require.True(t, iterator.Next())
				require.NoError(t, iterator.Err())
			})
		}
	})
}

func TestObserverGarbageCollection_MultipleRanges(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
		accessString, err := access.Serialize()
		require.NoError(t, err)

		for i := 0; i < 21; i++ {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bloomfilters", "object"+strconv.Itoa(i), testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)
		}

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)

		loopSegments := []rangedloop.Segment{}
		for _, segment := range segments {
			loopSegments = append(loopSegments, rangedloop.Segment{
				StreamID:      segment.StreamID,
				Position:      segment.Position,
				CreatedAt:     segment.CreatedAt,
				ExpiresAt:     segment.ExpiresAt,
				RepairedAt:    segment.RepairedAt,
				RootPieceID:   segment.RootPieceID,
				EncryptedSize: segment.EncryptedSize,
				PlainOffset:   segment.PlainOffset,
				PlainSize:     segment.PlainSize,
				Redundancy:    segment.Redundancy,
				Pieces:        segment.Pieces,
			})
		}

		// TODO: this is a little chicken-and-eggy... the GCBF config is
		// provided to the rangedloop service above, but we don't have the
		// access grant available until after testplanet has configured
		// everything. For now, just test the bloomfilter observer
		// directly, as is already done for the service. Maybe we can
		// improve this later.
		config := planet.Satellites[0].Config.GarbageCollectionBF
		config.AccessGrant = accessString
		config.Bucket = "bloomfilters"
		observers := []rangedloop.Observer{
			bloomfilter.NewObserver(zaptest.NewLogger(t), config, planet.Satellites[0].Overlay.DB),
			bloomfilter.NewSyncObserver(zaptest.NewLogger(t), config, planet.Satellites[0].Overlay.DB),
		}

		provider := &rangedlooptest.RangeSplitter{
			Segments: loopSegments,
		}

		rangedloopConfig := planet.Satellites[0].Config.RangedLoop
		rangedloopConfig.Parallelism = 5
		rangedloopConfig.BatchSize = 3

		for _, observer := range observers {
			name := fmt.Sprintf("%T", observer)
			t.Run(name, func(t *testing.T) {
				rangedLoop := rangedloop.NewService(zap.NewNop(), rangedloopConfig, provider,
					[]rangedloop.Observer{observer},
				)

				_, err = rangedLoop.RunOnce(ctx)
				require.NoError(t, err)
			})
		}
	})
}

func TestObserverGarbageCollection_CreationDate_MultipleSources(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	config := rangedloop.Config{Parallelism: 2, BatchSize: 3}

	testNodeID := testrand.NodeID()
	testPieces := metabase.Pieces{
		{Number: 1, StorageNode: testNodeID},
	}

	overlay := &mockOverlay{
		pieceCounts: map[storj.NodeID]int64{
			testNodeID: 1,
		},
	}

	segments := func(source string, creationTime time.Time, length int) (segments []rangedloop.Segment) {
		for i := 0; i < length; i++ {
			segments = append(segments, rangedloop.Segment{
				Pieces:      testPieces,
				CreatedAt:   creationTime,
				RootPieceID: testrand.PieceID(),
				Source:      source,
			})
		}
		return segments
	}

	concat := func(segments ...[]rangedloop.Segment) (result []rangedloop.Segment) {
		for _, segs := range segments {
			result = append(result, segs...)
		}
		return result
	}

	type testCase struct {
		Segments             []rangedloop.Segment
		ExpectedCreationTime time.Time
	}

	startTime := time.Now()
	for i, tc := range []testCase{
		{
			Segments:             segments("A", startTime.Add(time.Hour), 5),
			ExpectedCreationTime: startTime,
		},
		{
			Segments: concat(
				segments("A", startTime.Add(2*time.Hour), 5),
				segments("B", startTime.Add(3*time.Hour), 5),
				segments("C", startTime.Add(4*time.Hour), 5),
			),
			ExpectedCreationTime: startTime,
		},
		{
			Segments: concat(
				segments("A", startTime.Add(-time.Hour), 3),
				segments("B", startTime.Add(time.Hour), 3),
				segments("C", startTime.Add(2*time.Hour), 3),
			),
			ExpectedCreationTime: startTime.Add(-time.Hour),
		},
		{
			Segments: concat(
				segments("A", startTime.Add(-time.Hour), 6),
				segments("B", startTime.Add(-2*time.Hour), 6),
				segments("C", startTime.Add(-3*time.Hour), 6),
			),
			ExpectedCreationTime: startTime.Add(-3 * time.Hour),
		},
		{
			Segments: concat(
				segments("A", startTime.Add(-time.Hour), 3),
				segments("B", startTime.Add(-2*time.Hour), 3),
				segments("C", startTime.Add(-3*time.Hour), 3),
				segments("A", startTime.Add(time.Hour), 3),
				segments("B", startTime.Add(time.Hour), 3),
				segments("C", startTime.Add(time.Hour), 3),
			),
			ExpectedCreationTime: startTime,
		},
	} {
		provider := &rangedlooptest.RangeSplitter{Segments: tc.Segments}

		log := zaptest.NewLogger(t)
		observer := bloomfilter.NewObserver(log, bloomfilter.Config{AccessGrant: "test", Bucket: "test"}, overlay)

		rangedLoop := rangedloop.NewService(zap.NewNop(), config, provider,
			[]rangedloop.Observer{observer},
		)

		_, err := rangedLoop.RunOnce(ctx)
		require.NoError(t, err)
		require.WithinDuration(t, tc.ExpectedCreationTime, observer.TestingCreationTime(), time.Second, "case %d", i)
	}
}

type mockOverlay struct {
	pieceCounts map[storj.NodeID]int64
}

func (o *mockOverlay) ActiveNodesPieceCounts(ctx context.Context) (pieceCounts map[storj.NodeID]int64, err error) {
	return o.pieceCounts, nil
}
