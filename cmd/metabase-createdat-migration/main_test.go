// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/private/dbutil"
	"storj.io/private/dbutil/tempdb"
	migrator "storj.io/storj/cmd/metabase-createdat-migration"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
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

func TestMigrator_NoSegments(t *testing.T) {
	prepare := func(t *testing.T, ctx context.Context, rawDB *dbutil.TempDatabase, metabaseDB metainfo.MetabaseDB) {
		createObject(ctx, t, metabaseDB, 0)
	}

	check := func(t *testing.T, ctx context.Context, metabaseDB metainfo.MetabaseDB) {
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 0)
	}
	test(t, prepare, check)
}

func TestMigrator_SingleSegment(t *testing.T) {
	var expectedCreatedAt time.Time
	prepare := func(t *testing.T, ctx context.Context, rawDB *dbutil.TempDatabase, metabaseDB metainfo.MetabaseDB) {
		commitedObject := createObject(ctx, t, metabaseDB, 1)
		expectedCreatedAt = commitedObject.CreatedAt

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.NotNil(t, segments[0].CreatedAt)

		_, err = rawDB.ExecContext(ctx, `UPDATE segments SET created_at = NULL`)
		require.NoError(t, err)

		segments, err = metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.Nil(t, segments[0].CreatedAt)
	}

	check := func(t *testing.T, ctx context.Context, metabaseDB metainfo.MetabaseDB) {
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.NotNil(t, segments[0].CreatedAt)
		require.Equal(t, expectedCreatedAt, *segments[0].CreatedAt)
	}
	test(t, prepare, check)
}

func TestMigrator_ManySegments(t *testing.T) {
	numberOfObjects := 100
	expectedCreatedAt := map[uuid.UUID]time.Time{}

	prepare := func(t *testing.T, ctx context.Context, rawDB *dbutil.TempDatabase, metabaseDB metainfo.MetabaseDB) {
		for i := 0; i < numberOfObjects; i++ {
			commitedObject := createObject(ctx, t, metabaseDB, 1)
			expectedCreatedAt[commitedObject.StreamID] = commitedObject.CreatedAt
		}

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, numberOfObjects)
		for _, segment := range segments {
			require.NotNil(t, segment.CreatedAt)
		}

		_, err = rawDB.ExecContext(ctx, `UPDATE segments SET created_at = NULL`)
		require.NoError(t, err)

		segments, err = metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, numberOfObjects)
		for _, segment := range segments {
			require.Nil(t, segment.CreatedAt)
		}
	}

	check := func(t *testing.T, ctx context.Context, metabaseDB metainfo.MetabaseDB) {
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, numberOfObjects)
		for _, segment := range segments {
			require.NotNil(t, segment.CreatedAt)
			createdAt, found := expectedCreatedAt[segment.StreamID]
			require.True(t, found)
			require.Equal(t, createdAt, *segment.CreatedAt)
		}
	}
	test(t, prepare, check)
}

func TestMigrator_SegmentsWithAndWithoutCreatedAt(t *testing.T) {
	var expectedCreatedAt time.Time
	var segmentsBefore []metabase.Segment
	prepare := func(t *testing.T, ctx context.Context, rawDB *dbutil.TempDatabase, metabaseDB metainfo.MetabaseDB) {
		commitedObject := createObject(ctx, t, metabaseDB, 10)
		expectedCreatedAt = commitedObject.CreatedAt

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 10)
		for _, segment := range segments {
			require.NotNil(t, segment.CreatedAt)
		}

		// set created_at to null for half of segments
		_, err = rawDB.ExecContext(ctx, `UPDATE segments SET created_at = NULL WHERE position < 5`)
		require.NoError(t, err)

		segmentsBefore, err = metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segmentsBefore, 10)
		for i := 0; i < len(segmentsBefore); i++ {
			if i < 5 {
				require.Nil(t, segmentsBefore[i].CreatedAt)
			} else {
				require.NotNil(t, segmentsBefore[i].CreatedAt)
			}
		}
	}

	check := func(t *testing.T, ctx context.Context, metabaseDB metainfo.MetabaseDB) {
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 10)
		for i := 0; i < len(segments); i++ {
			require.NotNil(t, segments[i].CreatedAt)
			if i < 5 {
				require.Equal(t, expectedCreatedAt, *segments[i].CreatedAt)
			} else {
				require.NotEqual(t, expectedCreatedAt, segments[i].CreatedAt)
				require.Equal(t, segmentsBefore[i].CreatedAt, segments[i].CreatedAt)
			}
		}
	}
	test(t, prepare, check)
}

func test(t *testing.T, prepare func(t *testing.T, ctx context.Context, rawDB *dbutil.TempDatabase, metabaseDB metainfo.MetabaseDB),
	check func(t *testing.T, ctx context.Context, metabaseDB metainfo.MetabaseDB)) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	for _, satelliteDB := range satellitedbtest.Databases() {
		satelliteDB := satelliteDB
		if strings.EqualFold(satelliteDB.MasterDB.URL, "omit") {
			continue
		}
		t.Run(satelliteDB.Name, func(t *testing.T) {
			schemaSuffix := satellitedbtest.SchemaSuffix()
			schema := satellitedbtest.SchemaName(t.Name(), "category", 0, schemaSuffix)

			metabaseTempDB, err := tempdb.OpenUnique(ctx, satelliteDB.MetabaseDB.URL, schema)
			require.NoError(t, err)

			metabaseDB, err := satellitedbtest.CreateMetabaseDBOnTopOf(ctx, log, metabaseTempDB)
			require.NoError(t, err)
			defer ctx.Check(metabaseDB.Close)

			err = metabaseDB.MigrateToLatest(ctx)
			require.NoError(t, err)

			prepare(t, ctx, metabaseTempDB, metabaseDB)

			// TODO workaround for pgx
			mConnStr := strings.Replace(metabaseTempDB.ConnStr, "cockroach", "postgres", 1)
			err = migrator.Migrate(ctx, log, mConnStr, migrator.Config{
				LoopBatchSize:   40,
				UpdateBatchSize: 10,
			})
			require.NoError(t, err)

			check(t, ctx, metabaseDB)
		})
	}
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

func createObject(ctx context.Context, t *testing.T, metabaseDB metainfo.MetabaseDB, numberOfSegments int) metabase.Object {
	object, err := metabaseDB.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
		ObjectStream: randObjectStream(),
	})
	require.NoError(t, err)

	rootPieceID := testrand.PieceID()
	pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
	encryptedKey := testrand.Bytes(32)
	encryptedKeyNonce := testrand.Bytes(32)

	for i := 0; i < numberOfSegments; i++ {
		err = metabaseDB.BeginSegment(ctx, metabase.BeginSegment{
			ObjectStream: object.ObjectStream,
			Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
			RootPieceID:  rootPieceID,
			Pieces: []metabase.Piece{{
				Number:      1,
				StorageNode: testrand.NodeID(),
			}},
		})
		require.NoError(t, err)

		err = metabaseDB.CommitSegment(ctx, metabase.CommitSegment{
			ObjectStream: object.ObjectStream,
			Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
			RootPieceID:  rootPieceID,
			Pieces:       pieces,

			EncryptedKey:      encryptedKey,
			EncryptedKeyNonce: encryptedKeyNonce,

			EncryptedSize: 1024,
			PlainSize:     512,
			PlainOffset:   0,
			Redundancy:    defaultTestRedundancy,
		})
		require.NoError(t, err)
	}

	commitedObject, err := metabaseDB.CommitObject(ctx, metabase.CommitObject{
		ObjectStream: object.ObjectStream,
		Encryption:   defaultTestEncryption,
	})
	require.NoError(t, err)

	return commitedObject
}
