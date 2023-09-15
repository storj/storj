// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestIterateLoopObjects(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("Limit is negative", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BatchSize is negative",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("no data", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 0,
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 10,
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      10,
					AsOfSystemTime: time.Now(),
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("pending and committed", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pending := metabasetest.RandObjectStream()
			committed := metabasetest.RandObjectStream()
			committed.ProjectID = pending.ProjectID
			committed.BucketName = pending.BucketName + "z"

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: pending,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: committed,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:                  committed,
					OverrideEncryptedMetadata:     true,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			createdAt := time.Now()
			expected := []metabase.LoopObjectEntry{
				{
					ObjectStream: pending,
					Status:       metabase.Pending,
					CreatedAt:    createdAt,
				},
				{
					ObjectStream:          committed,
					Status:                metabase.Committed,
					EncryptedMetadataSize: len(encryptedMetadata),
					CreatedAt:             createdAt,
				},
			}

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 1,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      1,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("less objects than limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 3
			limit := 10
			expected := make([]metabase.LoopObjectEntry, numberOfObjects)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				expected[i] = loopObjectEntryFromRaw(obj)
			}

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: limit,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      limit,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("more objects than limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 10
			limit := 3
			expected := make([]metabase.LoopObjectEntry, numberOfObjects)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				expected[i] = loopObjectEntryFromRaw(obj)
			}

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: limit,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      limit,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("recursive", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			objects := metabasetest.CreateFullObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
				"a",
				"b/1",
				"b/2",
				"b/3",
				"c",
				"c/",
				"c//",
				"c/1",
				"g",
			})

			expected := []metabase.LoopObjectEntry{
				objects["a"],
				objects["b/1"],
				objects["b/2"],
				objects["b/3"],
				objects["c"],
				objects["c/"],
				objects["c//"],
				objects["c/1"],
				objects["g"],
			}

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 3,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      3,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("multiple projects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			projects := []uuid.UUID{}
			for i := 0; i < 10; i++ {
				p := testrand.UUID()
				p[0] = byte(i)
				projects = append(projects, p)
			}
			bucketNames := strings.Split("abcde", "")

			expected := make([]metabase.LoopObjectEntry, 0, len(projects)*len(bucketNames))
			for _, projectID := range projects {
				for _, bucketName := range bucketNames {
					rawObjects := createObjects(ctx, t, db, 1, projectID, bucketName)
					for _, obj := range rawObjects {
						expected = append(expected, loopObjectEntryFromRaw(obj))
					}
				}
			}

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 3,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      3,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("multiple projects multiple versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			projects := []uuid.UUID{}
			for i := 0; i < 10; i++ {
				p := testrand.UUID()
				p[0] = byte(i)
				projects = append(projects, p)
			}
			bucketNames := strings.Split("abcde", "")

			expected := make([]metabase.LoopObjectEntry, 0, len(projects)*len(bucketNames))
			for _, projectID := range projects {
				for _, bucketName := range bucketNames {
					obj := metabasetest.RandObjectStream()
					obj.ProjectID = projectID
					obj.BucketName = bucketName
					obj.Version = 1
					rawObject := metabasetest.CreateObject(ctx, t, db, obj, 0)
					expected = append(expected, loopObjectEntryFromRaw(metabase.RawObject(rawObject)))

					// pending objects
					for version := 2; version < 4; version++ {
						obj.Version = metabase.NextVersion
						rawObject, err := db.BeginObjectNextVersion(ctx, metabase.BeginObjectNextVersion{
							ObjectStream: obj,
						})
						require.NoError(t, err)

						expected = append(expected, loopPendingObjectEntryFromRaw(metabase.RawObject(rawObject)))
					}
				}
			}

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 2,
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      2,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)
		})
	})
}

func TestIterateLoopSegments(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {

		now := time.Now()

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
					BatchSize:      10,
					AsOfSystemTime: time.Now(),
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.IterateLoopSegments{
				Opts: metabase.IterateLoopSegments{
					BatchSize:      10,
					AsOfSystemTime: time.Now(),
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
					BatchSize:      10,
					AsOfSystemTime: time.Now(),
					StartStreamID:  startStreamID,
					EndStreamID:    endStreamID,
				},
				Result: nil,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("segments from pending and committed objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

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
					BatchSize:      1,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("batch size", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			numberOfSegments := 5

			committed := metabasetest.RandObjectStream()
			expectedObject := metabasetest.CreateObject(ctx, t, db, committed, byte(numberOfSegments))
			expected := make([]metabase.LoopSegmentEntry, numberOfSegments)
			expectedRaw := make([]metabase.RawSegment, numberOfSegments)
			for i := 0; i < numberOfSegments; i++ {
				entry := metabase.LoopSegmentEntry{
					StreamID:      committed.StreamID,
					Position:      metabase.SegmentPosition{0, uint32(i)},
					RootPieceID:   storj.PieceID{1},
					Pieces:        metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
					CreatedAt:     now,
					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   int64(i) * 512,
					Redundancy:    metabasetest.DefaultRedundancy,
					AliasPieces: metabase.AliasPieces([]metabase.AliasPiece{
						{Alias: 1},
					}),
				}
				expected[i] = entry
				expectedRaw[i] = metabase.RawSegment{
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

			{ // less segments than limit
				limit := 10
				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						BatchSize: limit,
					},
					Result: expected,
				}.Check(ctx, t, db)

				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						BatchSize:      limit,
						AsOfSystemTime: time.Now(),
					},
					Result: expected,
				}.Check(ctx, t, db)
			}

			{ // more segments than limit
				limit := 3
				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						BatchSize: limit,
					},
					Result: expected,
				}.Check(ctx, t, db)

				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						BatchSize:      limit,
						AsOfSystemTime: time.Now(),
					},
					Result: expected,
				}.Check(ctx, t, db)
			}

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(expectedObject),
				},
				Segments: expectedRaw,
			}.Check(ctx, t, db)
		})

		t.Run("streamID range", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			numberOfObjects := 10
			numberOfSegmentsPerObject := 3

			expected := make([]metabase.LoopSegmentEntry, numberOfObjects*numberOfSegmentsPerObject)
			expectedRaw := make([]metabase.RawSegment, numberOfObjects*numberOfSegmentsPerObject)
			expectedObjects := make([]metabase.RawObject, numberOfObjects)

			for i := 0; i < numberOfObjects; i++ {
				committed := metabasetest.RandObjectStream()

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
						AsOfSystemTime: time.Now(),
						StartStreamID:  expectedObjects[0].StreamID,
						EndStreamID:    expectedObjects[5].StreamID,
					},
					Result: expected[numberOfSegmentsPerObject : 6*numberOfSegmentsPerObject],
				}.Check(ctx, t, db)

				metabasetest.IterateLoopSegments{
					Opts: metabase.IterateLoopSegments{
						BatchSize:      1,
						AsOfSystemTime: time.Now(),
						StartStreamID:  expectedObjects[0].StreamID,
						EndStreamID:    expectedObjects[5].StreamID,
					},
					Result: expected[numberOfSegmentsPerObject : 6*numberOfSegmentsPerObject],
				}.Check(ctx, t, db)
			}

			metabasetest.Verify{
				Objects:  expectedObjects,
				Segments: expectedRaw,
			}.Check(ctx, t, db)
		})
	})
}

func loopObjectEntryFromRaw(m metabase.RawObject) metabase.LoopObjectEntry {
	return metabase.LoopObjectEntry{
		ObjectStream: m.ObjectStream,
		Status:       metabase.Committed,
		CreatedAt:    m.CreatedAt,
		ExpiresAt:    m.ExpiresAt,
		SegmentCount: m.SegmentCount,
	}
}

func loopPendingObjectEntryFromRaw(m metabase.RawObject) metabase.LoopObjectEntry {
	return metabase.LoopObjectEntry{
		ObjectStream: m.ObjectStream,
		Status:       metabase.Pending,
		CreatedAt:    m.CreatedAt,
		ExpiresAt:    m.ExpiresAt,
		SegmentCount: m.SegmentCount,
	}
}

func TestCollectBucketTallies(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("empty from", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					To: metabase.BucketLocation{
						ProjectID:  testrand.UUID(),
						BucketName: "name does not exist 2",
					},
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty to", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From: metabase.BucketLocation{
						ProjectID:  testrand.UUID(),
						BucketName: "name does not exist",
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "project ID To is before project ID From",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty bucket", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			randStream := metabasetest.RandObjectStream()

			obj := metabasetest.CreateObject(ctx, t, db, metabase.ObjectStream{
				ProjectID:  randStream.ProjectID,
				BucketName: randStream.BucketName,
				ObjectKey:  randStream.ObjectKey,
				Version:    randStream.Version,
				StreamID:   randStream.StreamID,
			}, 0)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					Version: randStream.Version,
					ObjectLocation: metabase.ObjectLocation{
						ProjectID:  randStream.ProjectID,
						BucketName: randStream.BucketName,
						ObjectKey:  randStream.ObjectKey,
					},
				},
				Result: metabase.DeleteObjectResult{Objects: []metabase.Object{obj}},
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					To: metabase.BucketLocation{
						ProjectID:  randStream.ProjectID,
						BucketName: randStream.BucketName,
					},
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty request", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From: metabase.BucketLocation{},
					To:   metabase.BucketLocation{},
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid bucket name", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			projectA := uuid.UUID{1}
			projectB := uuid.UUID{2}

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From: metabase.BucketLocation{
						ProjectID:  projectA,
						BucketName: "a\\",
					},
					To: metabase.BucketLocation{
						ProjectID:  projectB,
						BucketName: "b\\",
					},
				},
				Result: nil,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("pending and committed", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pending := metabasetest.RandObjectStream()
			committed := metabasetest.RandObjectStream()
			committed.ProjectID = pending.ProjectID
			committed.BucketName = pending.BucketName + "q"

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:                  pending,
					Encryption:                    metabasetest.DefaultEncryption,
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, committed, 1)

			expected := []metabase.BucketTally{
				{
					BucketLocation: metabase.BucketLocation{
						ProjectID:  pending.ProjectID,
						BucketName: pending.BucketName,
					},
					ObjectCount:        1,
					PendingObjectCount: 1,
					TotalSegments:      0,
					TotalBytes:         0,
					MetadataSize:       1024,
				},
				{
					BucketLocation: metabase.BucketLocation{
						ProjectID:  committed.ProjectID,
						BucketName: committed.BucketName,
					},
					ObjectCount:        1,
					PendingObjectCount: 0,
					TotalSegments:      1,
					TotalBytes:         1024,
					MetadataSize:       0,
				},
			}

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From: metabase.BucketLocation{
						ProjectID:  pending.ProjectID,
						BucketName: pending.BucketName,
					},
					To: metabase.BucketLocation{
						ProjectID:  committed.ProjectID,
						BucketName: committed.BucketName,
					},
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From: metabase.BucketLocation{
						ProjectID:  pending.ProjectID,
						BucketName: pending.BucketName,
					},
					To: metabase.BucketLocation{
						ProjectID:  committed.ProjectID,
						BucketName: committed.BucketName,
					},
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("multiple projects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			projects := []uuid.UUID{}
			for i := 0; i < 10; i++ {
				p := testrand.UUID()
				p[0] = byte(i)
				projects = append(projects, p)
			}
			bucketNames := strings.Split("abcde", "")
			bucketLocations := make([]metabase.BucketLocation, 0, len(projects)*len(bucketNames))

			expected := make([]metabase.BucketTally, 0, len(projects)*len(bucketNames))
			for _, projectID := range projects {
				for _, bucketName := range bucketNames {
					bucketLocations = append(bucketLocations, metabase.BucketLocation{
						ProjectID:  projectID,
						BucketName: bucketName,
					})
					rawObjects := createObjects(ctx, t, db, 1, projectID, bucketName)
					for _, obj := range rawObjects {
						expected = append(expected, bucketTallyFromRaw(obj))
					}
				}
			}
			sortBucketLocations(bucketLocations)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From: bucketLocations[0],
					To:   bucketLocations[len(bucketLocations)-1],
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:           bucketLocations[0],
					To:             bucketLocations[len(bucketLocations)-1],
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:           bucketLocations[0],
					To:             bucketLocations[15],
					AsOfSystemTime: time.Now(),
				},
				Result: expected[0:16],
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:           bucketLocations[16],
					To:             bucketLocations[34],
					AsOfSystemTime: time.Now(),
				},
				Result: expected[16:35],
			}.Check(ctx, t, db)

			metabasetest.CollectBucketTallies{
				Opts: metabase.CollectBucketTallies{
					From:           bucketLocations[30],
					To:             bucketLocations[10],
					AsOfSystemTime: time.Now(),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "project ID To is before project ID From",
			}.Check(ctx, t, db)
		})
	})
}

func bucketTallyFromRaw(m metabase.RawObject) metabase.BucketTally {
	return metabase.BucketTally{
		BucketLocation: metabase.BucketLocation{
			ProjectID:  m.ProjectID,
			BucketName: m.BucketName,
		},
		ObjectCount:   1,
		TotalSegments: int64(m.SegmentCount),
		TotalBytes:    m.TotalEncryptedSize,
		MetadataSize:  int64(len(m.EncryptedMetadata)),
	}
}

func sortBucketLocations(bc []metabase.BucketLocation) {
	sort.Slice(bc, func(i, j int) bool {
		if bc[i].ProjectID == bc[j].ProjectID {
			return bc[i].BucketName < bc[j].BucketName
		}
		return bc[i].ProjectID.Less(bc[j].ProjectID)
	})
}
