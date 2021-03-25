// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metainfo/metabase"
)

func TestGetObjectExactVersion(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		location := obj.Location()

		now := time.Now()

		for _, test := range invalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				GetObjectExactVersion{
					Opts: metabase.GetObjectExactVersion{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Version invalid", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        0,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version invalid: 0",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Get not existing version", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			createObject(ctx, t, db, obj, 0)

			GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        11,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get pending object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,

					Encryption: defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			createObject(ctx, t, db, obj, 0)

			GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				Result: metabase.Object{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
			}.Check(ctx, t, db)

			Verify{Objects: []metabase.RawObject{
				{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
			}}.Check(ctx, t, db)
		})
	})
}

func TestGetObjectLatestVersion(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		location := obj.Location()

		now := time.Now()

		for _, test := range invalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				GetObjectLatestVersion{
					Opts: metabase.GetObjectLatestVersion{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetObjectLatestVersion{
				Opts: metabase.GetObjectLatestVersion{
					ObjectLocation: location,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Get pending object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,

					Encryption: defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			GetObjectLatestVersion{
				Opts: metabase.GetObjectLatestVersion{
					ObjectLocation: location,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Run(ctx, t, db, obj, 0)

			GetObjectLatestVersion{
				Opts: metabase.GetObjectLatestVersion{
					ObjectLocation: location,
				},
				Result: metabase.Object{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,

					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			Verify{Objects: []metabase.RawObject{
				{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,

					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("Get latest object version from multiple", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			firstVersion := obj
			createObject(ctx, t, db, firstVersion, 0)
			secondVersion := metabase.ObjectStream{
				ProjectID:  obj.ProjectID,
				BucketName: obj.BucketName,
				ObjectKey:  obj.ObjectKey,
				Version:    2,
				StreamID:   obj.StreamID,
			}
			createObject(ctx, t, db, secondVersion, 0)

			GetObjectLatestVersion{
				Opts: metabase.GetObjectLatestVersion{
					ObjectLocation: location,
				},
				Result: metabase.Object{
					ObjectStream: secondVersion,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
			}.Check(ctx, t, db)

			Verify{Objects: []metabase.RawObject{
				{
					ObjectStream: firstVersion,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
				{
					ObjectStream: secondVersion,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
			}}.Check(ctx, t, db)
		})
	})
}

func TestGetSegmentByLocation(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		now := time.Now()

		location := metabase.SegmentLocation{
			ProjectID:  obj.ProjectID,
			BucketName: obj.BucketName,
			ObjectKey:  obj.ObjectKey,
		}

		for _, test := range invalidSegmentLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				GetSegmentByLocation{
					Opts: metabase.GetSegmentByLocation{
						SegmentLocation: test.SegmentLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetSegmentByLocation{
				Opts: metabase.GetSegmentByLocation{
					SegmentLocation: location,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: object or segment missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Get segment", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			createObject(ctx, t, db, obj, 1)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:         &now,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			GetSegmentByLocation{
				Opts: metabase.GetSegmentByLocation{
					SegmentLocation: location,
				},
				Result: expectedSegment,
			}.Check(ctx, t, db)

			// check non existing segment in existing object
			GetSegmentByLocation{
				Opts: metabase.GetSegmentByLocation{
					SegmentLocation: metabase.SegmentLocation{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Position: metabase.SegmentPosition{
							Index: 1,
						},
					},
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: object or segment missing",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						Encryption: defaultTestEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestGetSegmentByPosition(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		now := time.Now()

		t.Run("StreamID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetSegmentByPosition{
				Opts:     metabase.GetSegmentByPosition{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Segment missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: obj.StreamID,
				},
				ErrClass: &metabase.ErrSegmentNotFound,
				ErrText:  "segment missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Get segment", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			createObject(ctx, t, db, obj, 1)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:         &now,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: obj.StreamID,
					Position: metabase.SegmentPosition{
						Index: 0,
					},
				},
				Result: expectedSegment,
			}.Check(ctx, t, db)

			// check non existing segment in existing object
			GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: obj.StreamID,
					Position: metabase.SegmentPosition{
						Index: 1,
					},
				},
				ErrClass: &metabase.ErrSegmentNotFound,
				ErrText:  "segment missing",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						Encryption: defaultTestEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestGetLatestObjectLastSegment(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()
		location := obj.Location()
		now := time.Now()

		for _, test := range invalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				GetLatestObjectLastSegment{
					Opts: metabase.GetLatestObjectLastSegment{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object or segment missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: location,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: object or segment missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Get last segment", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			createObject(ctx, t, db, obj, 2)

			expectedSegmentSecond := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 1,
				},
				CreatedAt:         &now,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			expectedSegmentFirst := expectedSegmentSecond
			expectedSegmentFirst.Position.Index = 0

			GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: location,
				},
				Result: expectedSegmentSecond,
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 2,

						TotalPlainSize:     1024,
						TotalEncryptedSize: 2048,
						FixedSegmentSize:   512,

						Encryption: defaultTestEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegmentFirst),
					metabase.RawSegment(expectedSegmentSecond),
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestGetSegmentByOffset(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()
		location := obj.Location()
		now := time.Now()

		for _, test := range invalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				GetSegmentByOffset{
					Opts: metabase.GetSegmentByOffset{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Invalid PlainOffset", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetSegmentByOffset{
				Opts: metabase.GetSegmentByOffset{
					ObjectLocation: location,
					PlainOffset:    -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Invalid PlainOffset: -1",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Object or segment missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			GetSegmentByOffset{
				Opts: metabase.GetSegmentByOffset{
					ObjectLocation: location,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: object or segment missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Get segment", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			CreateTestObject{}.Run(ctx, t, db, obj, 4)

			segments := make([]metabase.Segment, 4)
			for i := range segments {
				segments[i] = metabase.Segment{
					StreamID: obj.StreamID,
					Position: metabase.SegmentPosition{
						Index: uint32(i),
					},
					CreatedAt:         &now,
					RootPieceID:       storj.PieceID{1},
					EncryptedKey:      []byte{3},
					EncryptedKeyNonce: []byte{4},
					EncryptedETag:     []byte{5},
					EncryptedSize:     1060,
					PlainSize:         512,
					PlainOffset:       int64(i * 512),
					Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
					Redundancy:        defaultTestRedundancy,
				}
			}

			var testCases = []struct {
				Offset          int64
				ExpectedSegment metabase.Segment
			}{
				{0, segments[0]},
				{100, segments[0]},
				{1023, segments[1]},
				{1024, segments[2]},
			}

			for _, tc := range testCases {
				GetSegmentByOffset{
					Opts: metabase.GetSegmentByOffset{
						ObjectLocation: location,
						PlainOffset:    tc.Offset,
					},
					Result: tc.ExpectedSegment,
				}.Check(ctx, t, db)
			}

			GetSegmentByOffset{
				Opts: metabase.GetSegmentByOffset{
					ObjectLocation: location,
					PlainOffset:    2048,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: object or segment missing",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 4,

						TotalPlainSize:     2048,
						TotalEncryptedSize: 4240,
						FixedSegmentSize:   512,

						Encryption: defaultTestEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(segments[0]),
					metabase.RawSegment(segments[1]),
					metabase.RawSegment(segments[2]),
					metabase.RawSegment(segments[3]),
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestBucketEmpty(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()
		now := time.Now()

		t.Run("ProjectID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BucketEmpty{
				Opts:     metabase.BucketEmpty{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "ProjectID missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("BucketName missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BucketEmpty{
				Opts: metabase.BucketEmpty{
					ProjectID: obj.ProjectID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BucketName missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("BucketEmpty true", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BucketEmpty{
				Opts: metabase.BucketEmpty{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
				},
				Result: true,
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("BucketEmpty false with pending object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,

					Encryption: defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			BucketEmpty{
				Opts: metabase.BucketEmpty{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
				},
				Result: false,
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,
						Encryption:   defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("BucketEmpty false with committed object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			object := createObject(ctx, t, db, obj, 0)

			BucketEmpty{
				Opts: metabase.BucketEmpty{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
				},
				Result: false,
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
			}.Check(ctx, t, db)
		})
	})
}
