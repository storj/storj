// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"math"
	"testing"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
)

var defaultTestRedundancy = storj.RedundancyScheme{
	Algorithm:      storj.ReedSolomon,
	ShareSize:      2048,
	RequiredShares: 1,
	RepairShares:   1,
	OptimalShares:  1,
	TotalShares:    1,
}

var defaultTestEncryption = storj.EncryptionParameters{
	CipherSuite: storj.EncAESGCM,
	BlockSize:   29 * 256,
}

func randObjectStream() metabase.ObjectStream {
	return metabase.ObjectStream{
		ProjectID:  testrand.UUID(),
		BucketName: testrand.BucketName(),
		ObjectKey:  metabase.ObjectKey(testrand.Bytes(16)),
		Version:    1,
		StreamID:   testrand.UUID(),
	}
}

type invalidObjectStream struct {
	Name         string
	ObjectStream metabase.ObjectStream
	ErrClass     *errs.Class
	ErrText      string
}

func invalidObjectStreams(base metabase.ObjectStream) []invalidObjectStream {
	var tests []invalidObjectStream
	{
		stream := base
		stream.ProjectID = uuid.UUID{}
		tests = append(tests, invalidObjectStream{
			Name:         "ProjectID missing",
			ObjectStream: stream,
			ErrClass:     &metabase.ErrInvalidRequest,
			ErrText:      "ProjectID missing",
		})
	}
	{
		stream := base
		stream.BucketName = ""
		tests = append(tests, invalidObjectStream{
			Name:         "BucketName missing",
			ObjectStream: stream,
			ErrClass:     &metabase.ErrInvalidRequest,
			ErrText:      "BucketName missing",
		})
	}
	{
		stream := base
		stream.ObjectKey = ""
		tests = append(tests, invalidObjectStream{
			Name:         "ObjectKey missing",
			ObjectStream: stream,
			ErrClass:     &metabase.ErrInvalidRequest,
			ErrText:      "ObjectKey missing",
		})
	}
	{
		stream := base
		stream.Version = -1
		tests = append(tests, invalidObjectStream{
			Name:         "Version invalid",
			ObjectStream: stream,
			ErrClass:     &metabase.ErrInvalidRequest,
			ErrText:      "Version invalid: -1",
		})
	}
	{
		stream := base
		stream.StreamID = uuid.UUID{}
		tests = append(tests, invalidObjectStream{
			Name:         "StreamID missing",
			ObjectStream: stream,
			ErrClass:     &metabase.ErrInvalidRequest,
			ErrText:      "StreamID missing",
		})
	}

	return tests
}

func TestBeginObjectNextVersion(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		for _, test := range invalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				BeginObjectNextVersion{
					Opts: metabase.BeginObjectNextVersion{
						ObjectStream: test.ObjectStream,
						Encryption:   defaultTestEncryption,
					},
					Version:  -1,
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		objectStream := metabase.ObjectStream{
			ProjectID:  obj.ProjectID,
			BucketName: obj.BucketName,
			ObjectKey:  obj.ObjectKey,
			StreamID:   obj.StreamID,
		}

		t.Run("disallow exact version", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			objectStream.Version = 5

			BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version:  -1,
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version should be metabase.NextVersion",
			}.Check(ctx, t, db)
		})

		t.Run("NextVersion", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

			objectStream.Version = metabase.NextVersion

			BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			now2 := time.Now()

			BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 2,
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    1,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Pending,

						Encryption: defaultTestEncryption,
					},
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    2,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now2,
						Status:    metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		// TODO: expires at date
		// TODO: zombie deletion deadline

		t.Run("older committed version exists", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			objectStream.Version = metabase.NextVersion

			BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    1,
						StreamID:   obj.StreamID,
					},
				},
			}.Check(ctx, t, db)

			now2 := time.Now()
			BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 2,
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    2,
						StreamID:   obj.StreamID,
					},
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    1,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Committed,

						Encryption: defaultTestEncryption,
					},
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    2,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now2,
						Status:    metabase.Committed,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("newer committed version exists", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

			objectStream.Version = metabase.NextVersion

			BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			now2 := time.Now()
			BeginObjectNextVersion{
				Opts: metabase.BeginObjectNextVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 2,
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    2,
						StreamID:   obj.StreamID,
					},
				},
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    1,
						StreamID:   obj.StreamID,
					},
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    1,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Committed,

						Encryption: defaultTestEncryption,
					},
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    2,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now2,
						Status:    metabase.Committed,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestBeginObjectExactVersion(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		for _, test := range invalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: test.ObjectStream,
						Encryption:   defaultTestEncryption,
					},
					Version:  -1,
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		objectStream := metabase.ObjectStream{
			ProjectID:  obj.ProjectID,
			BucketName: obj.BucketName,
			ObjectKey:  obj.ObjectKey,
			StreamID:   obj.StreamID,
		}

		t.Run("disallow NextVersion", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			objectStream.Version = metabase.NextVersion

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version:  -1,
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version should not be metabase.NextVersion",
			}.Check(ctx, t, db)
		})

		t.Run("Specific version", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

			objectStream.Version = 5

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 5,
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Duplicate pending version", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

			objectStream.Version = 5

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 5,
			}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version:  -1,
				ErrClass: &metabase.ErrConflict,
				ErrText:  "object already exists",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Duplicate committed version", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

			objectStream.Version = 5

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 5,
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
			}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version:  -1,
				ErrClass: &metabase.ErrConflict,
				ErrText:  "object already exists",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Committed,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})
		// TODO: expires at date
		// TODO: zombie deletion deadline

		t.Run("Older committed version exists", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

			objectStream.Version = 1

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
			}.Check(ctx, t, db)

			objectStream.Version = 3

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 3,
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    1,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Committed,

						Encryption: defaultTestEncryption,
					},
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    3,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Committed,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Newer committed version exists", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()

			objectStream.Version = 3

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 3,
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
			}.Check(ctx, t, db)

			objectStream.Version = 1

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objectStream,
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    1,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Committed,

						Encryption: defaultTestEncryption,
					},
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    3,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now1,
						Status:    metabase.Committed,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestBeginSegment(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		for _, test := range invalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				BeginSegment{
					Opts: metabase.BeginSegment{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("RootPieceID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "RootPieceID missing",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Pieces missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "pieces missing",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("StorageNode in pieces missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: storj.NodeID{},
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "piece number 1 is missing storage node id",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Piece number 2 is duplicated", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "duplicated piece number 1",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Pieces should be ordered", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
						{
							Number:      2,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "pieces should be ordered",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("pending object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.Error,
				ErrText:  "pending object missing",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("pending object missing when object committed", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			now := time.Now()

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.Error,
				ErrText:  "pending object missing",
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

		t.Run("begin segment successfully", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			now := time.Now()

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
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

		t.Run("multiple begin segment successfully", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)
			now := time.Now()

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			for i := 0; i < 5; i++ {
				BeginSegment{
					Opts: metabase.BeginSegment{
						ObjectStream: obj,
						RootPieceID:  storj.PieceID{1},
						Pieces: []metabase.Piece{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},
					},
				}.Check(ctx, t, db)
			}

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
	})
}

func TestCommitSegment(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()
		now := time.Now()

		for _, test := range invalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid request", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "RootPieceID missing",
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "pieces missing",
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: storj.NodeID{},
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "piece number 1 is missing storage node id",
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "duplicated piece number 1",
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
						{
							Number:      2,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "pieces should be ordered",
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  testrand.PieceID(),
					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKey missing",
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  testrand.PieceID(),

					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},

					EncryptedKey: testrand.Bytes(32),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKeyNonce missing",
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  testrand.PieceID(),

					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					EncryptedSize: -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedSize negative or zero",
			}.Check(ctx, t, db)

			if metabase.ValidatePlainSize {
				CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						RootPieceID:  testrand.PieceID(),

						Pieces: metabase.Pieces{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},

						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),

						EncryptedSize: 1024,
						PlainSize:     -1,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "PlainSize negative or zero",
				}.Check(ctx, t, db)
			}

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  testrand.PieceID(),

					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "PlainOffset negative",
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  testrand.PieceID(),

					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Redundancy zero",
			}.Check(ctx, t, db)

			redundancy := storj.RedundancyScheme{
				OptimalShares: 2,
			}

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
					RootPieceID:       testrand.PieceID(),
					Redundancy:        redundancy,
					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "number of pieces is less than redundancy optimal shares value",
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

		t.Run("duplicate", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,
				},
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    defaultTestRedundancy,
				},
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    defaultTestRedundancy,
				},
				ErrClass: &metabase.ErrConflict,
				ErrText:  "segment already exists",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now1,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 0, Index: 0},
						CreatedAt: &now,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: defaultTestRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit segment of missing object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    defaultTestRedundancy,
				},
				ErrClass: &metabase.Error,
				ErrText:  "pending object missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("commit segment of committed object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			now := time.Now()
			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    defaultTestRedundancy,
				},
				ErrClass: &metabase.Error,
				ErrText:  "pending object missing",
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

		t.Run("commit segment of pending object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)
			encryptedETag := testrand.Bytes(32)

			now := time.Now()
			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    defaultTestRedundancy,
					EncryptedETag: encryptedETag,
				},
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
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						CreatedAt: &now,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,
						EncryptedETag: encryptedETag,

						Redundancy: defaultTestRedundancy,

						Pieces: pieces,
					},
				}}.Check(ctx, t, db)
		})
	})
}

func TestCommitInlineSegment(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()
		now := time.Now()

		for _, test := range invalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid request", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKey missing",
			}.Check(ctx, t, db)

			CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					InlineData:   []byte{1, 2, 3},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKey missing",
			}.Check(ctx, t, db)

			CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,

					InlineData: []byte{1, 2, 3},

					EncryptedKey: testrand.Bytes(32),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKeyNonce missing",
			}.Check(ctx, t, db)

			if metabase.ValidatePlainSize {
				CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,

						InlineData: []byte{1, 2, 3},

						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),

						PlainSize: -1,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "PlainSize negative or zero",
				}.Check(ctx, t, db)
			}

			CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,

					InlineData: []byte{1, 2, 3},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					PlainSize:   512,
					PlainOffset: -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "PlainOffset negative",
			}.Check(ctx, t, db)
		})

		t.Run("duplicate", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					InlineData:   []byte{1, 2, 3},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:   512,
					PlainOffset: 0,
				},
			}.Check(ctx, t, db)

			CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					InlineData:   []byte{1, 2, 3},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:   512,
					PlainOffset: 0,
				},
				ErrClass: &metabase.ErrConflict,
				ErrText:  "segment already exists",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now1,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 0, Index: 0},
						CreatedAt: &now,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainOffset: 0,
						PlainSize:   512,

						InlineData:    []byte{1, 2, 3},
						EncryptedSize: 3,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit inline segment of missing object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					InlineData:   []byte{1, 2, 3},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:   512,
					PlainOffset: 0,
				},
				ErrClass: &metabase.Error,
				ErrText:  "pending object missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("commit segment of committed object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			now := time.Now()

			createObject(ctx, t, db, obj, 0)
			CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					InlineData:   []byte{1, 2, 3},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:   512,
					PlainOffset: 0,
				},
				ErrClass: &metabase.Error,
				ErrText:  "pending object missing",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						Encryption:   defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit empty segment of pending object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)
			encryptedETag := testrand.Bytes(32)

			now := time.Now()
			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:     0,
					PlainOffset:   0,
					EncryptedETag: encryptedETag,
				},
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
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						CreatedAt: &now,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainOffset: 0,
						PlainSize:   0,

						EncryptedSize: 0,
						EncryptedETag: encryptedETag,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit segment of pending object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)
			encryptedETag := testrand.Bytes(32)

			now := time.Now()
			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					InlineData:   []byte{1, 2, 3},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:     512,
					PlainOffset:   0,
					EncryptedETag: encryptedETag,
				},
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
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						CreatedAt: &now,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainOffset: 0,
						PlainSize:   512,

						InlineData:    []byte{1, 2, 3},
						EncryptedSize: 3,
						EncryptedETag: encryptedETag,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestCommitObject(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		for _, test := range invalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("version without pending", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: object with specified version and pending status is missing", // TODO: this error message could be better
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("version", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
					Encryption: defaultTestEncryption,
				},
				Version: 5,
			}.Check(ctx, t, db)
			now := time.Now()

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			// disallow for double commit
			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    5,
						StreamID:   obj.StreamID,
					},
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: object with specified version and pending status is missing", // TODO: this error message could be better
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  obj.ProjectID,
							BucketName: obj.BucketName,
							ObjectKey:  obj.ObjectKey,
							Version:    5,
							StreamID:   obj.StreamID,
						},
						CreatedAt: now,
						Status:    metabase.Committed,

						EncryptedMetadataNonce:        encryptedMetadataNonce[:],
						EncryptedMetadata:             encryptedMetadata,
						EncryptedMetadataEncryptedKey: encryptedMetadataKey,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("large object over 2 GB", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)
			now := time.Now()

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: math.MaxInt32,
					PlainSize:     math.MaxInt32,
					Redundancy:    defaultTestRedundancy,
				},
			}.Check(ctx, t, db)

			CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Index: 1},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: math.MaxInt32,
					PlainSize:     math.MaxInt32,
					Redundancy:    defaultTestRedundancy,
				},
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			Verify{
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Index: 0},
						CreatedAt: &now,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: math.MaxInt32,
						PlainSize:     math.MaxInt32,

						Redundancy: defaultTestRedundancy,

						Pieces: pieces,
					},
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Index: 1},
						CreatedAt: &now,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: math.MaxInt32,
						PlainSize:     math.MaxInt32,

						Redundancy: defaultTestRedundancy,

						Pieces: pieces,
					},
				},
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,

						SegmentCount:       2,
						FixedSegmentSize:   math.MaxInt32,
						TotalPlainSize:     2 * math.MaxInt32,
						TotalEncryptedSize: 2 * math.MaxInt32,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit with encryption", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
				},
				Version: 1,
			}.Check(ctx, t, db)

			now := time.Now()

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption:   storj.EncryptionParameters{},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Encryption is missing",
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption: storj.EncryptionParameters{
						CipherSuite: storj.EncAESGCM,
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Encryption.BlockSize is negative or zero",
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption: storj.EncryptionParameters{
						CipherSuite: storj.EncAESGCM,
						BlockSize:   -1,
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Encryption.BlockSize is negative or zero",
			}.Check(ctx, t, db)

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					Encryption: storj.EncryptionParameters{
						CipherSuite: storj.EncAESGCM,
						BlockSize:   512,
					},
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,

						SegmentCount: 0,

						Encryption: storj.EncryptionParameters{
							CipherSuite: storj.EncAESGCM,
							BlockSize:   512,
						},
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit with encryption (no override)", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			now := time.Now()

			CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
					// set different encryption than with BeginObjectExactVersion
					Encryption: storj.EncryptionParameters{
						CipherSuite: storj.EncNull,
						BlockSize:   512,
					},
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,

						SegmentCount: 0,
						Encryption:   defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestUpdateObjectMetadata(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()
		now := time.Now()

		for _, test := range invalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				UpdateObjectMetadata{
					Opts: metabase.UpdateObjectMetadata{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Version invalid", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateObjectMetadata{
				Opts: metabase.UpdateObjectMetadata{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    0,
						StreamID:   obj.StreamID,
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version invalid: 0",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateObjectMetadata{
				Opts: metabase.UpdateObjectMetadata{
					ObjectStream: obj,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: object with specified version and committed status is missing",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Update metadata", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			CreateTestObject{}.Run(ctx, t, db, obj, 0)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						Encryption:   defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)

			UpdateObjectMetadata{
				Opts: metabase.UpdateObjectMetadata{
					ObjectStream:                  obj,
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						Encryption:   defaultTestEncryption,

						EncryptedMetadata:             encryptedMetadata,
						EncryptedMetadataNonce:        encryptedMetadataNonce[:],
						EncryptedMetadataEncryptedKey: encryptedMetadataKey,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}
