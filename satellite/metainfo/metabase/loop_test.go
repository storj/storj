// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"bytes"
	"sort"
	"strings"
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
)

func TestIterateLoopObjects(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("Limit is negative", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BatchSize is negative",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("no data", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 0,
				},
				Result: nil,
			}.Check(ctx, t, db)

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 10,
				},
				Result: nil,
			}.Check(ctx, t, db)

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      10,
					AsOfSystemTime: time.Now(),
				},
				Result: nil,
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("pending and committed", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			pending := randObjectStream()
			committed := randObjectStream()
			committed.ProjectID = pending.ProjectID
			committed.BucketName = pending.BucketName + "z"

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: pending,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: committed,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:                  committed,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			expected := []metabase.LoopObjectEntry{
				{
					ObjectStream: pending,
				},
				{
					ObjectStream:          committed,
					EncryptedMetadataSize: len(encryptedMetadata),
				},
			}

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 1,
				},
				Result: expected,
			}.Check(ctx, t, db)

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      1,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("less objects than limit", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 3
			limit := 10
			expected := make([]metabase.LoopObjectEntry, numberOfObjects)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				expected[i] = loopObjectEntryFromRaw(obj)
			}

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: limit,
				},
				Result: expected,
			}.Check(ctx, t, db)

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      limit,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)

			Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("more objects than limit", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			numberOfObjects := 10
			limit := 3
			expected := make([]metabase.LoopObjectEntry, numberOfObjects)
			objects := createObjects(ctx, t, db, numberOfObjects, uuid.UUID{1}, "mybucket")
			for i, obj := range objects {
				expected[i] = loopObjectEntryFromRaw(obj)
			}

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: limit,
				},
				Result: expected,
			}.Check(ctx, t, db)

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      limit,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)

			Verify{Objects: objects}.Check(ctx, t, db)
		})

		t.Run("recursive", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			projectID, bucketName := uuid.UUID{1}, "bucky"

			objects := createFullObjectsWithKeys(ctx, t, db, projectID, bucketName, []metabase.ObjectKey{
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

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 3,
				},
				Result: expected,
			}.Check(ctx, t, db)

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      3,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("multiple projects", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

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

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 3,
				},
				Result: expected,
			}.Check(ctx, t, db)

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      3,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)
		})

		t.Run("multiple projects", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

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
					obj := randObjectStream()
					obj.ProjectID = projectID
					obj.BucketName = bucketName
					for version := 1; version < 4; version++ {
						obj.Version = metabase.Version(version)
						rawObject := createObject(ctx, t, db, obj, 0)
						expected = append(expected, loopObjectEntryFromRaw(metabase.RawObject(rawObject)))
					}
				}
			}

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize: 2,
				},
				Result: expected,
			}.Check(ctx, t, db)

			IterateLoopObjects{
				Opts: metabase.IterateLoopObjects{
					BatchSize:      2,
					AsOfSystemTime: time.Now(),
				},
				Result: expected,
			}.Check(ctx, t, db)
		})
	})
}

func TestIterateLoopStreams(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("StreamIDs list is empty", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			IterateLoopStreams{
				Opts:     metabase.IterateLoopStreams{},
				Result:   map[uuid.UUID][]metabase.LoopSegmentEntry{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamIDs list is empty",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("StreamIDs list contains empty ID", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			IterateLoopStreams{
				Opts: metabase.IterateLoopStreams{
					StreamIDs: []uuid.UUID{{}},
				},
				Result:   map[uuid.UUID][]metabase.LoopSegmentEntry{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing: index 0",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("List objects segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			expectedObject00 := createObject(ctx, t, db, randObjectStream(), 0)
			expectedObject01 := createObject(ctx, t, db, randObjectStream(), 1)
			expectedObject02 := createObject(ctx, t, db, randObjectStream(), 5)
			expectedObject03 := createObject(ctx, t, db, randObjectStream(), 3)

			expectedRawSegments := []metabase.RawSegment{}

			objects := []metabase.Object{
				expectedObject00,
				expectedObject01,
				expectedObject02,
				expectedObject03,
			}

			sort.Slice(objects, func(i, j int) bool {
				return bytes.Compare(objects[i].StreamID[:], objects[j].StreamID[:]) < 0
			})

			expectedMap := make(map[uuid.UUID][]metabase.LoopSegmentEntry)
			for _, object := range objects {
				var expectedSegments []metabase.LoopSegmentEntry
				for i := 0; i < int(object.SegmentCount); i++ {
					segment := metabase.LoopSegmentEntry{
						StreamID: object.StreamID,
						Position: metabase.SegmentPosition{
							Index: uint32(i),
						},
						CreatedAt:     &now,
						RootPieceID:   storj.PieceID{1},
						EncryptedSize: 1024,
						Pieces:        metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
						Redundancy:    defaultTestRedundancy,
					}
					expectedSegments = append(expectedSegments, segment)

					expectedRawSegments = append(expectedRawSegments, metabase.RawSegment{
						StreamID:          segment.StreamID,
						Position:          segment.Position,
						CreatedAt:         &now,
						EncryptedSize:     segment.EncryptedSize,
						Pieces:            segment.Pieces,
						Redundancy:        segment.Redundancy,
						RootPieceID:       segment.RootPieceID,
						PlainSize:         512,
						PlainOffset:       int64(i * 512),
						EncryptedKey:      []byte{3},
						EncryptedKeyNonce: []byte{4},
						EncryptedETag:     []byte{5},
					})
				}
				expectedMap[object.StreamID] = expectedSegments
			}

			IterateLoopStreams{
				Opts: metabase.IterateLoopStreams{
					StreamIDs: []uuid.UUID{
						expectedObject00.StreamID,
						expectedObject01.StreamID,
						expectedObject02.StreamID,
						expectedObject03.StreamID,
					},

					AsOfSystemTime: time.Now(),
				},
				Result: expectedMap,
			}.Check(ctx, t, db)

			IterateLoopStreams{
				Opts: metabase.IterateLoopStreams{
					StreamIDs: []uuid.UUID{
						expectedObject00.StreamID,
						expectedObject01.StreamID,
						expectedObject02.StreamID,
						expectedObject03.StreamID,
					},
				},
				Result: expectedMap,
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(expectedObject00),
					metabase.RawObject(expectedObject01),
					metabase.RawObject(expectedObject02),
					metabase.RawObject(expectedObject03),
				},
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)
		})
	})
}

func createFullObjectsWithKeys(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, bucketName string, keys []metabase.ObjectKey) map[metabase.ObjectKey]metabase.LoopObjectEntry {
	objects := make(map[metabase.ObjectKey]metabase.LoopObjectEntry, len(keys))
	for _, key := range keys {
		obj := randObjectStream()
		obj.ProjectID = projectID
		obj.BucketName = bucketName
		obj.ObjectKey = key

		createObject(ctx, t, db, obj, 0)

		objects[key] = metabase.LoopObjectEntry{
			ObjectStream: obj,
		}
	}

	return objects
}

func loopObjectEntryFromRaw(m metabase.RawObject) metabase.LoopObjectEntry {
	return metabase.LoopObjectEntry{
		ObjectStream: m.ObjectStream,
		ExpiresAt:    m.ExpiresAt,
		SegmentCount: m.SegmentCount,
	}
}
