// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"strings"
	"testing"
	"time"

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
				Version: 1,
			}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: committed,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
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
					obj := metabasetest.RandObjectStream()
					obj.ProjectID = projectID
					obj.BucketName = bucketName
					for version := 1; version < 4; version++ {
						obj.Version = metabase.Version(version)
						rawObject := metabasetest.CreateObject(ctx, t, db, obj, 0)
						expected = append(expected, loopObjectEntryFromRaw(metabase.RawObject(rawObject)))
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
