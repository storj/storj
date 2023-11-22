// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

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
				_, err := planet.Satellites[0].DB.Testing().RawDB().ExecContext(ctx, "UPDATE nodes SET disqualified = null WHERE id = $1", node.ID())
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
					segments := rangedloop.NewMetabaseRangeSplitter(planet.Satellites[0].Metabase.DB, rangedloopConfig.AsOfSystemInterval, rangedloopConfig.BatchSize)
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
		observer := bloomfilter.NewObserver(zaptest.NewLogger(t), config, planet.Satellites[0].Overlay.DB)

		// TODO: see comment above. ideally this should use the rangedloop
		// service instantiated for the testplanet.
		rangedloopConfig := planet.Satellites[0].Config.RangedLoop
		segments := rangedloop.NewMetabaseRangeSplitter(planet.Satellites[0].Metabase.DB, rangedloopConfig.AsOfSystemInterval, rangedloopConfig.BatchSize)
		rangedLoop := rangedloop.NewService(zap.NewNop(), planet.Satellites[0].Config.RangedLoop, segments,
			[]rangedloop.Observer{observer})

		_, err = rangedLoop.RunOnce(ctx)
		require.NoError(t, err)

		// check that there are 2 objects and the names match
		iterator := project.ListObjects(ctx, "bloomfilters", nil)
		keys := []string{}
		for iterator.Next() {
			if !iterator.Item().IsPrefix {
				keys = append(keys, iterator.Item().Key)
			}
		}
		require.Len(t, keys, 2)
		require.Contains(t, keys, "some object")
		require.Contains(t, keys, bloomfilter.LATEST)
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

func BenchmarkProcess(b *testing.B) {
	ctx := context.Background()

	nodes := make([]storj.NodeID, 10)
	pieceCount := make(map[storj.NodeID]int64, len(nodes))
	for i := range nodes {
		nodes[i] = testrand.NodeID()
		pieceCount[nodes[i]] = 0
	}

	numberOfSegments := 1000
	segments := make([]rangedloop.Segment, numberOfSegments)
	for i := range segments {
		segments[i].RootPieceID = testrand.PieceID()

		// part := i % 10
		// for j := 0; j < 10; j++ {
		// 	segments[i].Pieces = append(segments[i].Pieces, metabase.Piece{
		// 		Number:      uint16(j),
		// 		StorageNode: nodes[part+j],
		// 	})
		// }

		for j, node := range nodes {
			segments[i].Pieces = append(segments[i].Pieces, metabase.Piece{
				Number:      uint16(j),
				StorageNode: node,
			})
		}
	}

	overlay := Overlay{
		piecesCount: pieceCount,
	}

	log := zap.NewNop()
	config := bloomfilter.Config{
		InitialPieces:     400000,
		FalsePositiveRate: 0.1,
		AccessGrant:       "test",
		Bucket:            "test",
	}

	observer := bloomfilter.NewObserver(log, config, &overlay)
	syncObserver := bloomfilter.NewSyncObserver(log, config, &overlay)
	syncObserver2 := bloomfilter.NewSyncObserver2(log, config, &overlay)

	benchmarks := map[string]rangedloop.Observer{
		"non sync": observer,
		"sync":     syncObserver,
		"sync2":    syncObserver2,
	}

	for name, observer := range benchmarks {
		require.NoError(b, observer.Start(ctx, time.Now()))

		forks := make([]rangedloop.Partial, 5)
		for i := range forks {
			var err error
			forks[i], err = observer.Fork(ctx)
			require.NoError(b, err)
		}

		b.Run(name, func(b *testing.B) {
			segmentsRangeSize := len(segments) / len(forks)
			group := errgroup.Group{}
			for i := 0; i < b.N; i++ {
				for i, fork := range forks {
					fork := fork
					r := segments[i*segmentsRangeSize : i*segmentsRangeSize+segmentsRangeSize]
					group.Go(func() error {
						return fork.Process(ctx, r)
					})
				}
				require.NoError(b, group.Wait())
			}
		})
	}
}

type Overlay struct {
	piecesCount map[storj.NodeID]int64
}

func (o *Overlay) ActiveNodesPieceCounts(ctx context.Context) (pieceCounts map[storj.NodeID]int64, err error) {
	return o.piecesCount, nil
}
