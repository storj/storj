// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestIterateLoopSegments(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("Limit is negative", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					BatchSize: -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BatchSize is negative",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Wrongly defined ranges", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			startStreamID, err := uuid.New()
			require.NoError(t, err)

			endStreamID, err := uuid.New()
			require.NoError(t, err)

			if startStreamID.Less(endStreamID) {
				startStreamID, endStreamID = endStreamID, startStreamID
			}

			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					StartStreamID: startStreamID,
					EndStreamID:   endStreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EndStreamID is smaller than StartStreamID",
			}.Check(ctx, t, db)

			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					StartStreamID: startStreamID,
					EndStreamID:   startStreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StartStreamID and EndStreamID must be different",
			}.Check(ctx, t, db)
			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					StartStreamID: startStreamID,
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("no segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					BatchSize: 0,
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					BatchSize: 10,
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					BatchSize: 10,
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					BatchSize: 10,
				},
				Result: nil,
			}.Check(ctx, t, db)

			startStreamID, err := uuid.New()
			require.NoError(t, err)

			endStreamID, err := uuid.New()
			require.NoError(t, err)

			if endStreamID.Less(startStreamID) {
				startStreamID, endStreamID = endStreamID, startStreamID
			}

			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					BatchSize:     10,
					StartStreamID: startStreamID,
					EndStreamID:   endStreamID,
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("segments from pending and committed objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			pending := metabasetest.RandObjectStream()
			metabasetest.CreatePendingObject(ctx, t, db, pending, 2)

			committed := metabasetest.RandObjectStream()
			metabasetest.CreateObject(ctx, t, db, committed, 3)

			expectedExpiresAt := now.Add(33 * time.Hour)
			committedExpires := metabasetest.RandObjectStream()
			metabasetest.CreateExpiredObject(ctx, t, db, committedExpires, 1, expectedExpiresAt)

			genericLoopEntry := metabase.LoopSegmentEntry{
				RootPieceID:   storj.PieceID{1},
				Pieces:        metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				CreatedAt:     now,
				EncryptedSize: 1024,
				PlainSize:     512,
				PlainOffset:   0,
				Redundancy:    metabasetest.DefaultRedundancy,
			}

			expectedSource := db.ChooseAdapter(committed.ProjectID).Name()

			expected := []metabase.LoopSegmentEntry{}
			for _, expect := range []struct {
				StreamID    uuid.UUID
				Position    metabase.SegmentPosition
				PlainOffset int64
				ExpiresAt   *time.Time
			}{
				{pending.StreamID, metabase.SegmentPosition{0, 0}, 0, nil},
				{pending.StreamID, metabase.SegmentPosition{0, 1}, 0, nil},
				{committed.StreamID, metabase.SegmentPosition{0, 0}, 0, nil},
				{committed.StreamID, metabase.SegmentPosition{0, 1}, 512, nil},
				{committed.StreamID, metabase.SegmentPosition{0, 2}, 1024, nil},
				{committedExpires.StreamID, metabase.SegmentPosition{0, 0}, 0, &expectedExpiresAt},
			} {
				entry := genericLoopEntry
				entry.StreamID = expect.StreamID
				entry.Position = expect.Position
				entry.PlainOffset = expect.PlainOffset
				entry.ExpiresAt = expect.ExpiresAt
				entry.AliasPieces = metabase.AliasPieces([]metabase.AliasPiece{
					{Alias: 1},
				})
				entry.Source = expectedSource
				expected = append(expected, entry)
			}

			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					BatchSize: 1,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					BatchSize: 1,
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("streamID range", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			numberOfObjects := 10
			numberOfSegmentsPerObject := 3

			expected := make([]metabase.LoopSegmentEntry, numberOfObjects*numberOfSegmentsPerObject)
			expectedRaw := make([]metabase.RawSegment, numberOfObjects*numberOfSegmentsPerObject)
			expectedObjects := make([]metabase.RawObject, numberOfObjects)

			for i := 0; i < numberOfObjects; i++ {
				committed := metabasetest.RandObjectStream()

				source := db.ChooseAdapter(committed.ProjectID).Name()

				expectedObjects[i] = metabase.RawObject(
					metabasetest.CreateObject(ctx, t, db, committed, byte(numberOfSegmentsPerObject)))

				for j := 0; j < numberOfSegmentsPerObject; j++ {

					entry := metabase.LoopSegmentEntry{
						StreamID:      committed.StreamID,
						Position:      metabase.SegmentPosition{0, uint32(j)},
						RootPieceID:   storj.PieceID{1},
						Pieces:        metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
						CreatedAt:     now,
						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   int64(j) * 512,
						Redundancy:    metabasetest.DefaultRedundancy,
						AliasPieces: metabase.AliasPieces([]metabase.AliasPiece{
							{Alias: 1},
						}),
						Source: source,
					}
					expected[i*numberOfSegmentsPerObject+j] = entry
					expectedRaw[i*numberOfSegmentsPerObject+j] = metabase.RawSegment{
						StreamID:      entry.StreamID,
						Position:      entry.Position,
						RootPieceID:   entry.RootPieceID,
						Pieces:        entry.Pieces,
						CreatedAt:     entry.CreatedAt,
						EncryptedSize: entry.EncryptedSize,
						PlainSize:     entry.PlainSize,
						PlainOffset:   entry.PlainOffset,
						Redundancy:    entry.Redundancy,

						EncryptedKey:      []byte{3},
						EncryptedKeyNonce: []byte{4},
						EncryptedETag:     []byte{5},
					}
				}
			}
			sort.Slice(expected, func(i, j int) bool {
				if expected[i].StreamID.Less(expected[j].StreamID) {
					return true
				}
				if expected[i].StreamID == expected[j].StreamID {
					return expected[i].Position.Less(expected[j].Position)
				}
				return false
			})

			sort.Slice(expectedObjects, func(i, j int) bool {
				return expectedObjects[i].StreamID.Less(expectedObjects[j].StreamID)
			})

			{ // StartStreamID set
				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						StartStreamID: expectedObjects[0].StreamID,
					},
					Result: expected[numberOfSegmentsPerObject:],
				}.Check(ctx, t, db)

				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						StartStreamID: expectedObjects[0].StreamID,
						BatchSize:     1,
					},
					Result: expected[numberOfSegmentsPerObject:],
				}.Check(ctx, t, db)
			}

			{ // EndStreamID set
				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						EndStreamID: expectedObjects[3].StreamID,
					},
					Result: expected[:4*numberOfSegmentsPerObject],
				}.Check(ctx, t, db)

				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						BatchSize:   1,
						EndStreamID: expectedObjects[3].StreamID,
					},
					Result: expected[:4*numberOfSegmentsPerObject],
				}.Check(ctx, t, db)

				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						BatchSize:   1,
						EndStreamID: expectedObjects[numberOfObjects-1].StreamID,
					},
					Result: expected,
				}.Check(ctx, t, db)
			}

			{ // StartStreamID and EndStreamID set
				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						StartStreamID: expectedObjects[0].StreamID,
						EndStreamID:   expectedObjects[5].StreamID,
					},
					Result: expected[numberOfSegmentsPerObject : 6*numberOfSegmentsPerObject],
				}.Check(ctx, t, db)

				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						BatchSize:     1,
						StartStreamID: expectedObjects[0].StreamID,
						EndStreamID:   expectedObjects[5].StreamID,
					},
					Result: expected[numberOfSegmentsPerObject : 6*numberOfSegmentsPerObject],
				}.Check(ctx, t, db)
			}

			metabasetest.Verify{
				Objects:  expectedObjects,
				Segments: expectedRaw,
			}.Check(ctx, t, db)
		})

		t.Run("check segment source", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 1)
			expectedSource := db.ChooseAdapter(object.ProjectID).Name()

			err := db.IterateLoopSegments(ctx, metabase.IterateLoopSegments{
				BatchSize: 1,
			}, func(ctx context.Context, lsi metabase.LoopSegmentsIterator) error {

				var entry metabase.LoopSegmentEntry
				for lsi.Next(ctx, &entry) {
					require.Equal(t, expectedSource, entry.Source)
				}
				return nil
			})
			require.NoError(t, err)
		})

		t.Run("batch size", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			segments := []metabase.RawSegment{}
			expectedSegments := []metabase.LoopSegmentEntry{}
			expectedSource := db.ChooseAdapter(uuid.UUID{}).Name()
			for i := 0; i < 10; i++ {
				segment := metabasetest.DefaultRawSegment(metabasetest.RandObjectStream(), metabase.SegmentPosition{})
				segments = append(segments, segment)
				expectedSegments = append(expectedSegments, metabase.LoopSegmentEntry{
					StreamID:      segment.StreamID,
					Position:      segment.Position,
					CreatedAt:     segment.CreatedAt,
					ExpiresAt:     segment.ExpiresAt,
					RepairedAt:    segment.RepairedAt,
					RootPieceID:   segment.RootPieceID,
					EncryptedSize: segment.EncryptedSize,
					PlainOffset:   segment.PlainOffset,
					PlainSize:     segment.PlainSize,
					Pieces:        segment.Pieces,
					Redundancy:    segment.Redundancy,
					Placement:     segment.Placement,
					Source:        expectedSource,
				})
			}

			err := db.TestingBatchInsertSegments(ctx, segments)
			require.NoError(t, err)

			for _, batchSize := range []int{0, 1, 2, 3, 8, 9, 2000} {
				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						BatchSize: batchSize,
					},
					Result: expectedSegments,
				}.Check(ctx, t, db)
			}
		})
	})
}
