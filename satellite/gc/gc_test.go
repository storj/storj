// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc_test

import (
	"context"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/rangedloop/rangedlooptest"
	"storj.io/storj/storagenode"
	"storj.io/uplink/private/testuplink"
)

type observerConfiguration struct {
	name string
	f    func(*zap.Logger, bloomfilter.Config, bloomfilter.Overlay) rangedloop.Observer
}

func observerConfigurations() []observerConfiguration {
	configurations := []observerConfiguration{
		{
			name: "SyncObserverV2",
			f: func(log *zap.Logger, config bloomfilter.Config, overlay bloomfilter.Overlay) rangedloop.Observer {
				return bloomfilter.NewSyncObserverV2(log, config, overlay)
			},
		},
		{
			name: "SyncObserver",
			f: func(log *zap.Logger, config bloomfilter.Config, overlay bloomfilter.Overlay) rangedloop.Observer {
				return bloomfilter.NewSyncObserver(log, config, overlay)
			},
		},
		{
			name: "Observer",
			f: func(log *zap.Logger, config bloomfilter.Config, overlay bloomfilter.Overlay) rangedloop.Observer {
				return bloomfilter.NewObserver(log, config, overlay)
			},
		},
	}

	rand.Shuffle(len(configurations), func(i, j int) {
		configurations[i], configurations[j] = configurations[j], configurations[i]
	})

	return configurations
}

func testObservers(t *testing.T, run func(*testing.T, func(*zap.Logger, bloomfilter.Config, bloomfilter.Overlay) rangedloop.Observer)) {
	for _, tt := range observerConfigurations() {
		t.Run(tt.name, func(t *testing.T) { run(t, tt.f) })
	}
}

func benchmarkObservers(b *testing.B, run func(*testing.B, func(*zap.Logger, bloomfilter.Config, bloomfilter.Overlay) rangedloop.Observer)) {
	for _, tt := range observerConfigurations() {
		b.Run(tt.name, func(b *testing.B) { run(b, tt.f) })
	}
}

// TestGarbageCollection does the following:
// * Set up a network with one storagenode
// * Upload two objects
// * Delete one object from the metainfo service on the satellite
// * Do bloom filter generation
// * Send out bloom filters
// * Check that pieces of the deleted object are deleted on the storagenode
// * Check that pieces of the kept object are not deleted on the storagenode.
func TestGarbageCollection(t *testing.T) {
	testObservers(t, func(t *testing.T, makeObserver func(*zap.Logger, bloomfilter.Config, bloomfilter.Overlay) rangedloop.Observer) {
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				StorageNode: func(index int, config *storagenode.Config) {
					config.Retain.MaxTimeSkew = -time.Minute
				},
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			// Set satellite 1 to store bloom filters of satellite 0
			access := planet.Uplinks[0].Access[planet.Satellites[1].NodeURL().ID]
			accessString, err := access.Serialize()
			require.NoError(t, err)

			// configure sender
			gcsender := planet.Satellites[0].GarbageCollection.Sender
			gcsender.Config.AccessGrant = accessString

			// configure filter uploader
			config := planet.Satellites[0].Config.GarbageCollectionBF
			config.AccessGrant = accessString

			satellite := planet.Satellites[0]
			upl := planet.Uplinks[0]
			targetNode := planet.StorageNodes[0]

			// Upload two objects
			testDataKeep := testrand.Bytes(8 * memory.KiB)
			testDataDelete := testrand.Bytes(9 * memory.KiB)

			require.NoError(t, upl.Upload(ctx, satellite, "testbucket", "test/path/keep", testDataKeep))
			require.NoError(t, upl.Upload(ctx, satellite, "testbucket", "test/path/delete", testDataDelete))

			segments, err := satellite.Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 2)

			sort.Slice(segments, func(i, j int) bool {
				return segments[i].CreatedAt.Before(segments[j].CreatedAt)
			})

			segmentToKeep := segments[0]
			segmentToDelete := segments[1]

			findPiece := func(segment metabase.Segment) storj.PieceID {
				for _, p := range segment.Pieces {
					if p.StorageNode == targetNode.ID() {
						return segment.RootPieceID.Derive(p.StorageNode, int32(p.Number))
					}
				}
				require.Fail(t, "piece id not found")
				return storj.PieceID{}
			}

			keptPieceID := findPiece(segmentToKeep)
			deletedPieceID := findPiece(segmentToDelete)

			require.NoError(t, upl.DeleteObject(ctx, satellite, "testbucket", "test/path/delete"))

			// Check that piece of the deleted object is on the storagenode
			r, err := targetNode.Storage2.PieceBackend.Reader(ctx, satellite.ID(), deletedPieceID)
			require.NoError(t, err)
			require.False(t, r.Trash())
			require.NoError(t, r.Close())

			// Wait for bloom filter observer to finish
			rangedloopConfig := planet.Satellites[0].Config.RangedLoop

			observer := makeObserver(zap.NewNop(), config, satellite.Overlay.DB)
			mbSegments := rangedloop.NewMetabaseRangeSplitter(zap.NewNop(), planet.Satellites[0].Metabase.DB, rangedloopConfig)
			rangedLoop := rangedloop.NewService(zap.NewNop(), planet.Satellites[0].Config.RangedLoop, mbSegments, []rangedloop.Observer{observer})

			_, err = rangedLoop.RunOnce(ctx)
			require.NoError(t, err)

			// send to storagenode
			err = gcsender.RunOnce(ctx)
			require.NoError(t, err)

			// Wait for the storagenode's RetainService queue to be empty
			targetNode.StorageOld.RetainService.TestWaitUntilEmpty()
			require.NoError(t, targetNode.Storage2.HashStoreBackend.TestingCompact(ctx))

			// Check that piece of the deleted object is trashed
			r, err = targetNode.Storage2.PieceBackend.Reader(ctx, satellite.ID(), deletedPieceID)
			require.NoError(t, err)
			require.True(t, r.Trash())
			require.NoError(t, r.Close())

			// Check that piece of the kept object is on the storagenode
			r, err = targetNode.Storage2.PieceBackend.Reader(ctx, satellite.ID(), keptPieceID)
			require.NoError(t, err)
			require.False(t, r.Trash())
			require.NoError(t, r.Close())
		})
	})
}

// TestGarbageCollectionWithCopies checkes that server-side copy elements are not
// affecting GC and nothing unexpected was deleted from storage nodes.
func TestGarbageCollectionWithCopies(t *testing.T) {
	t.Skip("flaky")
	testObservers(t, func(t *testing.T, makeObserver func(*zap.Logger, bloomfilter.Config, bloomfilter.Overlay) rangedloop.Observer) {
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]

			access := planet.Uplinks[0].Access[planet.Satellites[0].NodeURL().ID]
			accessString, err := access.Serialize()
			require.NoError(t, err)

			gcsender := planet.Satellites[0].GarbageCollection.Sender
			gcsender.Config.AccessGrant = accessString

			// configure filter uploader
			config := planet.Satellites[0].Config.GarbageCollectionBF
			config.AccessGrant = accessString

			project, err := planet.Uplinks[0].OpenProject(ctx, satellite)
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			allSpaceUsedForPieces := func() (all int64) {
				for _, node := range planet.StorageNodes {
					space, err := node.Storage2.SpaceReport.DiskSpace(ctx)
					require.NoError(t, err)
					all += space.UsedForPieces
				}
				return all
			}

			expectedRemoteData := testrand.Bytes(8 * memory.KiB)
			expectedInlineData := testrand.Bytes(1 * memory.KiB)

			require.NoError(t, planet.Uplinks[0].Upload(ctx, satellite, "testbucket", "remote", expectedRemoteData))
			require.NoError(t, planet.Uplinks[0].Upload(ctx, satellite, "testbucket", "inline", expectedInlineData))
			require.NoError(t, planet.Uplinks[0].Upload(ctx, satellite, "testbucket", "remote-no-copy", expectedRemoteData))

			_, err = project.CopyObject(ctx, "testbucket", "remote", "testbucket", "remote-copy", nil)
			require.NoError(t, err)
			_, err = project.CopyObject(ctx, "testbucket", "inline", "testbucket", "inline-copy", nil)
			require.NoError(t, err)

			require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

			spaceUsedAfterUpload := allSpaceUsedForPieces()

			// Wait for bloom filter observer to finish
			rangedloopConfig := planet.Satellites[0].Config.RangedLoop

			observer := makeObserver(zap.NewNop(), config, satellite.Overlay.DB)
			segments := rangedloop.NewMetabaseRangeSplitter(zap.NewNop(), planet.Satellites[0].Metabase.DB, rangedloopConfig)
			rangedLoop := rangedloop.NewService(zap.NewNop(), planet.Satellites[0].Config.RangedLoop, segments, []rangedloop.Observer{observer})

			_, err = rangedLoop.RunOnce(ctx)
			require.NoError(t, err)

			// send to storagenode
			require.NoError(t, gcsender.RunOnce(ctx))

			for _, node := range planet.StorageNodes {
				node.StorageOld.RetainService.TestWaitUntilEmpty()
				require.NoError(t, node.Storage2.HashStoreBackend.TestingCompact(ctx))
			}

			// we should see all space used by all objects
			require.Equal(t, spaceUsedAfterUpload, allSpaceUsedForPieces())

			for _, toDelete := range []string{
				// delete ancestors, no change in used space
				"remote",
				"inline",
				// delete object without copy, used space should be decreased
				"remote-no-copy",
			} {
				_, err = project.DeleteObject(ctx, "testbucket", toDelete)
				require.NoError(t, err)
			}

			// run GC
			_, err = rangedLoop.RunOnce(ctx)
			require.NoError(t, err)

			// send to storagenode
			require.NoError(t, gcsender.RunOnce(ctx))

			for _, node := range planet.StorageNodes {
				node.StorageOld.RetainService.TestWaitUntilEmpty()
				require.NoError(t, node.Storage2.HashStoreBackend.TestingCompact(ctx))
			}

			// verify that we deleted only pieces for "remote-no-copy" object
			spaceUsedAfterFirstGC := allSpaceUsedForPieces()
			require.LessOrEqual(t, spaceUsedAfterFirstGC, spaceUsedAfterUpload)

			// delete rest of objects to verify that everything will be removed also from SNs
			for _, toDelete := range []string{
				"remote-copy",
				"inline-copy",
			} {
				_, err = project.DeleteObject(ctx, "testbucket", toDelete)
				require.NoError(t, err)
			}

			// run GC
			_, err = rangedLoop.RunOnce(ctx)
			require.NoError(t, err)

			// send to storagenode
			require.NoError(t, gcsender.RunOnce(ctx))

			for _, node := range planet.StorageNodes {
				node.StorageOld.RetainService.TestWaitUntilEmpty()
				require.NoError(t, node.Storage2.HashStoreBackend.TestingCompact(ctx))
			}

			// verify that nothing more was deleted from storage nodes after GC
			spaceUsedAfterSecondGC := allSpaceUsedForPieces()
			require.LessOrEqual(t, spaceUsedAfterSecondGC, spaceUsedAfterFirstGC)
		})
	})
}

// TestGarbageCollectionWithCopies checks that server-side copy elements are not
// affecting GC and nothing unexpected was deleted from storage nodes.
func TestGarbageCollectionWithCopiesWithDuplicateMetadata(t *testing.T) {
	t.Skip("flaky")

	testObservers(t, func(t *testing.T, makeObserver func(*zap.Logger, bloomfilter.Config, bloomfilter.Overlay) rangedloop.Observer) {
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
			Reconfigure: testplanet.Reconfigure{
				Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]

			access := planet.Uplinks[0].Access[planet.Satellites[0].NodeURL().ID]
			accessString, err := access.Serialize()
			require.NoError(t, err)

			gcsender := planet.Satellites[0].GarbageCollection.Sender
			gcsender.Config.AccessGrant = accessString

			// configure filter uploader
			config := planet.Satellites[0].Config.GarbageCollectionBF
			config.AccessGrant = accessString

			project, err := planet.Uplinks[0].OpenProject(ctx, satellite)
			require.NoError(t, err)
			defer ctx.Check(project.Close)

			allSpaceUsedForPieces := func() (all int64) {
				for _, node := range planet.StorageNodes {
					space, err := node.Storage2.SpaceReport.DiskSpace(ctx)
					require.NoError(t, err)
					all += space.UsedForPieces
				}
				return all
			}

			expectedRemoteData := testrand.Bytes(8 * memory.KiB)
			expectedInlineData := testrand.Bytes(1 * memory.KiB)

			require.EqualValues(t, 0, allSpaceUsedForPieces())

			require.NoError(t, planet.Uplinks[0].Upload(ctx, satellite, "testbucket", "remote", expectedRemoteData))
			require.NoError(t, planet.Uplinks[0].Upload(ctx, satellite, "testbucket", "inline", expectedInlineData))
			require.NoError(t, planet.Uplinks[0].Upload(ctx, satellite, "testbucket", "remote-no-copy", expectedRemoteData))

			_, err = project.CopyObject(ctx, "testbucket", "remote", "testbucket", "remote-copy", nil)
			require.NoError(t, err)
			_, err = project.CopyObject(ctx, "testbucket", "inline", "testbucket", "inline-copy", nil)
			require.NoError(t, err)

			require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

			spaceUsedAfterUpload := allSpaceUsedForPieces()

			// Wait for bloom filter observer to finish
			rangedloopConfig := planet.Satellites[0].Config.RangedLoop

			observer := makeObserver(zap.NewNop(), config, satellite.Overlay.DB)
			segments := rangedloop.NewMetabaseRangeSplitter(zap.NewNop(), planet.Satellites[0].Metabase.DB, rangedloopConfig)
			rangedLoop := rangedloop.NewService(zap.NewNop(), planet.Satellites[0].Config.RangedLoop, segments, []rangedloop.Observer{observer})

			_, err = rangedLoop.RunOnce(ctx)
			require.NoError(t, err)

			// send to storagenode
			require.NoError(t, gcsender.RunOnce(ctx))

			for _, node := range planet.StorageNodes {
				node.StorageOld.RetainService.TestWaitUntilEmpty()
				require.NoError(t, node.Storage2.HashStoreBackend.TestingCompact(ctx))
			}

			// we should see all space used by all objects
			spaceUsedAfterFirstGC := allSpaceUsedForPieces()
			require.LessOrEqual(t, spaceUsedAfterFirstGC, spaceUsedAfterUpload)

			for _, toDelete := range []string{
				// delete ancestors, no change in used space
				"remote",
				"inline",
				// delete object without copy, used space should be decreased
				"remote-no-copy",
			} {
				_, err = project.DeleteObject(ctx, "testbucket", toDelete)
				require.NoError(t, err)
			}

			// run GC
			_, err = rangedLoop.RunOnce(ctx)
			require.NoError(t, err)

			// send to storagenode
			require.NoError(t, gcsender.RunOnce(ctx))

			for _, node := range planet.StorageNodes {
				node.StorageOld.RetainService.TestWaitUntilEmpty()
				require.NoError(t, node.Storage2.HashStoreBackend.TestingCompact(ctx))
			}

			// verify that we deleted only pieces for "remote-no-copy" object
			require.Less(t, allSpaceUsedForPieces(), spaceUsedAfterUpload)

			// delete rest of objects to verify that everything will be removed also from SNs
			for _, toDelete := range []string{
				"remote-copy",
				"inline-copy",
			} {
				_, err = project.DeleteObject(ctx, "testbucket", toDelete)
				require.NoError(t, err)
			}

			// run GC
			_, err = rangedLoop.RunOnce(ctx)
			require.NoError(t, err)

			// send to storagenode
			require.NoError(t, gcsender.RunOnce(ctx))

			for _, node := range planet.StorageNodes {
				node.StorageOld.RetainService.TestWaitUntilEmpty()
				require.NoError(t, node.Storage2.HashStoreBackend.TestingCompact(ctx))
			}

			// verify that nothing more was deleted from storage nodes after GC
			spaceUsedAfterSecondGC := allSpaceUsedForPieces()
			require.LessOrEqual(t, spaceUsedAfterSecondGC, spaceUsedAfterFirstGC)
		})
	})
}

// TestGarbageCollection_PendingObject verifies that segments from pending objects
// are also processed by GC piece tracker.
func TestGarbageCollection_PendingObject(t *testing.T) {
	testObservers(t, func(t *testing.T, makeObserver func(*zap.Logger, bloomfilter.Config, bloomfilter.Overlay) rangedloop.Observer) {
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
			accessString, err := access.Serialize()
			require.NoError(t, err)

			satellite := planet.Satellites[0]
			upl := planet.Uplinks[0]

			testData := testrand.Bytes(15 * memory.KiB)
			pendingStreamID := startMultipartUpload(ctx, t, upl, satellite, "testbucket", "multi", testData)

			segments, err := satellite.Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 1)

			config := planet.Satellites[0].Config.GarbageCollectionBF
			config.AccessGrant = accessString
			config.Bucket = "bucket"
			config.FalsePositiveRate = 0.000000001
			config.InitialPieces = 10

			observer := makeObserver(zap.NewNop(), config, satellite.Overlay.DB)
			rangedloopConfig := planet.Satellites[0].Config.RangedLoop
			provider := rangedloop.NewMetabaseRangeSplitter(zap.NewNop(), planet.Satellites[0].Metabase.DB, rangedloopConfig)
			rangedLoop := rangedloop.NewService(zap.NewNop(), planet.Satellites[0].Config.RangedLoop, provider, []rangedloop.Observer{observer})

			_, err = rangedLoop.RunOnce(ctx)
			require.NoError(t, err)

			testingObserver, ok := observer.(bloomfilter.TestingObserver)
			require.True(t, ok)
			require.NotEmpty(t, testingObserver.TestingRetainInfos())
			info, ok := testingObserver.TestingRetainInfos().Load(planet.StorageNodes[0].ID())
			require.True(t, ok)
			require.NotNil(t, info)
			require.Equal(t, 1, info.Count)

			completeMultipartUpload(ctx, t, upl, satellite, "testbucket", "multi", pendingStreamID)
			gotData, err := upl.Download(ctx, satellite, "testbucket", "multi")
			require.NoError(t, err)
			require.Equal(t, testData, gotData)
		})
	})
}

func startMultipartUpload(ctx context.Context, t *testing.T, uplink *testplanet.Uplink, satellite *testplanet.Satellite, bucketName string, path storj.Path, data []byte) string {
	_, found := testuplink.GetMaxSegmentSize(ctx)
	if !found {
		ctx = testuplink.WithMaxSegmentSize(ctx, satellite.Config.Metainfo.MaxSegmentSize)
	}

	project, err := uplink.GetProject(ctx, satellite)
	require.NoError(t, err)
	defer func() { require.NoError(t, project.Close()) }()

	_, err = project.EnsureBucket(ctx, bucketName)
	require.NoError(t, err)

	info, err := project.BeginUpload(ctx, bucketName, path, nil)
	require.NoError(t, err)

	upload, err := project.UploadPart(ctx, bucketName, path, info.UploadID, 1)
	require.NoError(t, err)
	_, err = upload.Write(data)
	require.NoError(t, err)
	require.NoError(t, upload.Commit())

	return info.UploadID
}

func completeMultipartUpload(ctx context.Context, t *testing.T, uplink *testplanet.Uplink, satellite *testplanet.Satellite, bucketName string, path storj.Path, streamID string) {
	_, found := testuplink.GetMaxSegmentSize(ctx)
	if !found {
		ctx = testuplink.WithMaxSegmentSize(ctx, satellite.Config.Metainfo.MaxSegmentSize)
	}

	project, err := uplink.GetProject(ctx, satellite)
	require.NoError(t, err)
	defer func() { require.NoError(t, project.Close()) }()

	_, err = project.CommitUpload(ctx, bucketName, path, streamID, nil)
	require.NoError(t, err)
}

func BenchmarkGarbageCollection(b *testing.B) {
	const (
		storageNodesCount     = 100
		segmentsCount         = 10000
		piecesPerSegment      = 10
		rangedLoopBatchSize   = 2500
		rangedLoopParallelism = 40
	)

	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	log := zap.NewNop()
	defer ctx.Check(log.Sync)

	segments, pieceCounts := randomSegments(storageNodesCount, segmentsCount, piecesPerSegment)

	bfConfig := bloomfilter.Config{
		AccessGrant: "access",
		Bucket:      "bucket",
	}

	overlay := &mockOverlay{pieceCounts: pieceCounts}

	rlConfig := rangedloop.Config{
		BatchSize:   rangedLoopBatchSize,
		Parallelism: rangedLoopParallelism,
	}
	provider := &rangedlooptest.RangeSplitter{
		Segments: segments,
	}

	benchmarkObservers(b, func(b *testing.B, makeObserver func(*zap.Logger, bloomfilter.Config, bloomfilter.Overlay) rangedloop.Observer) {
		observer := makeObserver(log, bfConfig, overlay)
		rangedLoop := rangedloop.NewService(log, rlConfig, provider, []rangedloop.Observer{observer})

		durations := make(map[rangedloop.Observer]time.Duration)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			d, err := rangedLoop.RunOnce(ctx)
			require.NoError(b, err)

			for _, d := range d {
				durations[d.Observer] += d.Duration
			}
		}
		observerDurations = durations

		require.Len(b, observerDurations, 1)

		// for _, d := range observerDurations {
		// 	b.ReportMetric(float64(d.Nanoseconds())/float64(b.N), "ns/RunOnce")
		// }
	})
}

var observerDurations map[rangedloop.Observer]time.Duration

type mockOverlay struct {
	pieceCounts map[storj.NodeID]int64
}

func (o *mockOverlay) ActiveNodesPieceCounts(context.Context) (map[storj.NodeID]int64, error) {
	return o.pieceCounts, nil
}

func randomSegments(nodesCount, segmentsCount, piecesPerSegment int) ([]rangedloop.Segment, map[storj.NodeID]int64) {
	var nodes []storj.NodeID
	for i := 0; i < nodesCount; i++ {
		nodes = append(nodes, testrand.NodeID())
	}

	pieceCounts := make(map[storj.NodeID]int64)

	startDate := time.Date(2000, time.August, 8, 0, 0, 0, 0, time.UTC)
	var segments []rangedloop.Segment
	for i := 0; i < segmentsCount; i++ {
		var pieces []metabase.Piece
		for j := 0; j < piecesPerSegment; j++ {
			node := nodes[(i+j)%len(nodes)]
			pieces = append(pieces, metabase.Piece{
				Number:      uint16(j),
				StorageNode: node,
			})
			pieceCounts[node]++
		}
		segments = append(segments, rangedloop.Segment{
			CreatedAt:   startDate.Add(time.Hour * time.Duration(i)),
			RootPieceID: testrand.PieceID(),
			Pieces:      pieces,
		})
	}

	return segments, pieceCounts
}
