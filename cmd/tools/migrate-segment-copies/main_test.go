// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	cmd "storj.io/storj/cmd/tools/migrate-segment-copies"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestMigrateSingleCopy(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, metabaseDB *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		expectedPieces := metabase.Pieces{
			{Number: 1, StorageNode: testrand.NodeID()},
			{Number: 3, StorageNode: testrand.NodeID()},
		}

		object, _ := metabasetest.CreateTestObject{
			CreateSegment: func(object metabase.Object, index int) metabase.Segment {
				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: uint32(index)},
						RootPieceID:  testrand.PieceID(),

						Pieces: expectedPieces,

						EncryptedKey:      []byte{3},
						EncryptedKeyNonce: []byte{4},
						EncryptedETag:     []byte{5},

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
						Redundancy:    metabasetest.DefaultRedundancy,
						Placement:     storj.EEA,
					},
				}.Check(ctx, t, metabaseDB)

				return metabase.Segment{}
			},
		}.Run(ctx, t, metabaseDB, obj, 50)

		copyObject, _, _ := metabasetest.CreateObjectCopy{
			OriginalObject: object,
		}.Run(ctx, t, metabaseDB, false)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		for _, segment := range segments {
			if segment.StreamID == copyObject.StreamID {
				require.Len(t, segment.Pieces, 0)
				require.Equal(t, storj.EveryCountry, segment.Placement)
			}
		}

		require.NotZero(t, numberOfSegmentCopies(t, ctx, metabaseDB))

		err = cmd.MigrateSegments(ctx, zaptest.NewLogger(t), metabaseDB, cmd.Config{
			BatchSize: 3,
		})
		require.NoError(t, err)

		segments, err = metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		for _, segment := range segments {
			require.Equal(t, expectedPieces, segment.Pieces)
			require.Equal(t, storj.EEA, segment.Placement)
		}
	})
}

func TestMigrateManyCopies(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, metabaseDB *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		expectedPieces := metabase.Pieces{
			{Number: 1, StorageNode: testrand.NodeID()},
			{Number: 3, StorageNode: testrand.NodeID()},
		}

		object, _ := metabasetest.CreateTestObject{
			CreateSegment: func(object metabase.Object, index int) metabase.Segment {
				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						Position:     metabase.SegmentPosition{Part: 0, Index: uint32(index)},
						RootPieceID:  testrand.PieceID(),

						Pieces: expectedPieces,

						EncryptedKey:      []byte{3},
						EncryptedKeyNonce: []byte{4},
						EncryptedETag:     []byte{5},

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
						Redundancy:    metabasetest.DefaultRedundancy,
						Placement:     storj.EEA,
					},
				}.Check(ctx, t, metabaseDB)

				return metabase.Segment{}
			},
		}.Run(ctx, t, metabaseDB, obj, 20)

		for i := 0; i < 10; i++ {
			copyObject, _, _ := metabasetest.CreateObjectCopy{
				OriginalObject: object,
			}.Run(ctx, t, metabaseDB, false)

			segments, err := metabaseDB.TestingAllSegments(ctx)
			require.NoError(t, err)
			for _, segment := range segments {
				if segment.StreamID == copyObject.StreamID {
					require.Len(t, segment.Pieces, 0)
					require.Equal(t, storj.EveryCountry, segment.Placement)
				}
			}
		}

		require.NotZero(t, numberOfSegmentCopies(t, ctx, metabaseDB))

		err := cmd.MigrateSegments(ctx, zaptest.NewLogger(t), metabaseDB, cmd.Config{
			BatchSize: 7,
		})
		require.NoError(t, err)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		for _, segment := range segments {
			require.Equal(t, expectedPieces, segment.Pieces)
			require.Equal(t, storj.EEA, segment.Placement)
		}
	})
}

func TestMigrateDifferentSegment(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, metabaseDB *metabase.DB) {
		type Segment struct {
			StreamID uuid.UUID
			Position int64
		}

		expectedResults := map[Segment]metabase.Pieces{}
		createData := func(numberOfObjecsts int, pieces metabase.Pieces) {
			for i := 0; i < numberOfObjecsts; i++ {
				numberOfSegments := 3
				obj := metabasetest.RandObjectStream()
				object, _ := metabasetest.CreateTestObject{
					CreateSegment: func(object metabase.Object, index int) metabase.Segment {
						metabasetest.CommitSegment{
							Opts: metabase.CommitSegment{
								ObjectStream: obj,
								Position:     metabase.SegmentPosition{Part: 0, Index: uint32(index)},
								RootPieceID:  testrand.PieceID(),

								Pieces: pieces,

								EncryptedKey:      []byte{3},
								EncryptedKeyNonce: []byte{4},
								EncryptedETag:     []byte{5},

								EncryptedSize: 1024,
								PlainSize:     512,
								PlainOffset:   0,
								Redundancy:    metabasetest.DefaultRedundancy,
								Placement:     storj.EEA,
							},
						}.Check(ctx, t, metabaseDB)

						return metabase.Segment{}
					},
				}.Run(ctx, t, metabaseDB, obj, 3)
				for n := 0; n < numberOfSegments; n++ {
					expectedResults[Segment{
						StreamID: object.StreamID,
						Position: int64(n),
					}] = pieces
				}

				copyObject, _, _ := metabasetest.CreateObjectCopy{
					OriginalObject: object,
				}.Run(ctx, t, metabaseDB, false)

				for n := 0; n < numberOfSegments; n++ {
					expectedResults[Segment{
						StreamID: copyObject.StreamID,
						Position: int64(n),
					}] = pieces

					segments, err := metabaseDB.TestingAllSegments(ctx)
					require.NoError(t, err)
					for _, segment := range segments {
						if segment.StreamID == copyObject.StreamID {
							require.Len(t, segment.Pieces, 0)
							require.Equal(t, storj.EveryCountry, segment.Placement)
						}
					}
				}
			}
		}

		expectedPieces := metabase.Pieces{
			{Number: 1, StorageNode: testrand.NodeID()},
			{Number: 3, StorageNode: testrand.NodeID()},
		}
		createData(5, expectedPieces)

		expectedPieces = metabase.Pieces{
			{Number: 2, StorageNode: testrand.NodeID()},
			{Number: 4, StorageNode: testrand.NodeID()},
		}
		createData(5, expectedPieces)

		require.NotZero(t, numberOfSegmentCopies(t, ctx, metabaseDB))

		err := cmd.MigrateSegments(ctx, zaptest.NewLogger(t), metabaseDB, cmd.Config{
			BatchSize: 7,
		})
		require.NoError(t, err)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Equal(t, len(expectedResults), len(segments))
		for _, segment := range segments {
			pieces := expectedResults[Segment{
				StreamID: segment.StreamID,
				Position: int64(segment.Position.Encode()),
			}]
			require.Equal(t, pieces, segment.Pieces)
			require.Equal(t, storj.EEA, segment.Placement)
		}
	})
}

func numberOfSegmentCopies(t *testing.T, ctx *testcontext.Context, metabaseDB *metabase.DB) int {
	var count int
	err := metabaseDB.UnderlyingTagSQL().QueryRow(ctx, "SELECT count(1) FROM segment_copies").Scan(&count)
	require.NoError(t, err)
	return count
}

func TestMigrateEndToEnd(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedData := testrand.Bytes(10 * memory.KiB)
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test", "object", expectedData)
		require.NoError(t, err)

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		_, err = project.CopyObject(ctx, "test", "object", "test", "object-copy", nil)
		require.NoError(t, err)

		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "test", "object-copy")
		require.NoError(t, err)
		require.Equal(t, expectedData, data)

		err = cmd.MigrateSegments(ctx, zaptest.NewLogger(t), planet.Satellites[0].Metabase.DB, cmd.Config{
			BatchSize: 1,
		})
		require.NoError(t, err)

		data, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "test", "object-copy")
		require.NoError(t, err)
		require.Equal(t, expectedData, data)
	})
}

func TestMigrateBackupCSV(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedData := testrand.Bytes(10 * memory.KiB)
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test", "object", expectedData)
		require.NoError(t, err)

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		_, err = project.CopyObject(ctx, "test", "object", "test", "object-copy", nil)
		require.NoError(t, err)

		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "test", "object-copy")
		require.NoError(t, err)
		require.Equal(t, expectedData, data)

		backupFile := ctx.File("backupcsv")
		err = cmd.MigrateSegments(ctx, zaptest.NewLogger(t), planet.Satellites[0].Metabase.DB, cmd.Config{
			BatchSize:           1,
			SegmentCopiesBackup: backupFile,
		})
		require.NoError(t, err)

		data, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "test", "object-copy")
		require.NoError(t, err)
		require.Equal(t, expectedData, data)

		fileByes, err := os.ReadFile(backupFile)
		require.NoError(t, err)
		require.NotEmpty(t, fileByes)
	})
}
