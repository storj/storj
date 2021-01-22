// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	migration "storj.io/storj/cmd/metainfo-migration"
	"storj.io/storj/private/dbutil/tempdb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage"
)

func TestMigrator_SingleSegmentObj(t *testing.T) {
	expectedEntries := 1
	createPointers := func(t *testing.T, ctx context.Context, pointerDB metainfo.PointerDB) {
		projectID := testrand.UUID()
		err := createLastSegment(ctx, pointerDB, projectID, []byte("bucket-name"), []byte("encrypted-key"), 1)
		require.NoError(t, err)
	}

	checkMigration := func(t *testing.T, ctx context.Context, metabaseDB metainfo.MetabaseDB) {
		objects, err := metabaseDB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, expectedEntries)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Equal(t, len(segments), expectedEntries)

		require.EqualValues(t, 0, segments[0].Position.Part)
		require.EqualValues(t, 0, segments[0].Position.Index)

	}
	test(t, createPointers, checkMigration)
}

func TestMigrator_ManyOneSegObj(t *testing.T) {
	expectedEntries := 1000
	createPointers := func(t *testing.T, ctx context.Context, pointerDB metainfo.PointerDB) {
		projectID := testrand.UUID()
		for i := 0; i < expectedEntries; i++ {
			err := createLastSegment(ctx, pointerDB, projectID, []byte("bucket-name"), []byte("encrypted-key"+strconv.Itoa(i)), 1)
			require.NoError(t, err)
		}
	}

	checkMigration := func(t *testing.T, ctx context.Context, metabaseDB metainfo.MetabaseDB) {
		objects, err := metabaseDB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, expectedEntries)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Equal(t, len(segments), expectedEntries)
	}
	test(t, createPointers, checkMigration)
}

func TestMigrator_MultiSegmentObj(t *testing.T) {
	expectedEntries := 1000
	createPointers := func(t *testing.T, ctx context.Context, pointerDB metainfo.PointerDB) {
		projectID := testrand.UUID()

		err := createLastSegment(ctx, pointerDB, projectID, []byte("bucket-name"), []byte("encrypted-key"), expectedEntries+1)
		require.NoError(t, err)
		for i := 0; i < expectedEntries; i++ {
			err = createSegment(ctx, pointerDB, projectID, uint32(i), []byte("bucket-name"), []byte("encrypted-key"))
			require.NoError(t, err)
		}
	}

	checkMigration := func(t *testing.T, ctx context.Context, metabaseDB metainfo.MetabaseDB) {
		objects, err := metabaseDB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 1)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Equal(t, len(segments), expectedEntries+1)
	}
	test(t, createPointers, checkMigration)
}

func test(t *testing.T, createPointers func(t *testing.T, ctx context.Context, pointerDB metainfo.PointerDB), checkMigration func(t *testing.T, ctx context.Context, metabaseDB metainfo.MetabaseDB)) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	for _, satelliteDB := range satellitedbtest.Databases() {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.Name, func(t *testing.T) {
			schemaSuffix := satellitedbtest.SchemaSuffix()
			schema := satellitedbtest.SchemaName(t.Name(), "category", 0, schemaSuffix)
			pointerTempDB, err := tempdb.OpenUnique(ctx, satelliteDB.PointerDB.URL, schema)
			require.NoError(t, err)

			pointerDB, err := satellitedbtest.CreatePointerDBOnTopOf(ctx, log, pointerTempDB)
			require.NoError(t, err)
			defer ctx.Check(pointerDB.Close)

			metabaseTempDB, err := tempdb.OpenUnique(ctx, satelliteDB.MetabaseDB.URL, schema)
			require.NoError(t, err)
			metabaseDB, err := satellitedbtest.CreateMetabaseDBOnTopOf(ctx, log, metabaseTempDB)
			require.NoError(t, err)
			defer ctx.Check(metabaseDB.Close)

			createPointers(t, ctx, pointerDB)

			// TODO workaround for pgx
			pConnStr := strings.Replace(pointerTempDB.ConnStr, "cockroach", "postgres", 1)
			mConnStr := strings.Replace(metabaseTempDB.ConnStr, "cockroach", "postgres", 1)
			migrator := migration.NewMigrator(log, pConnStr, mConnStr, migration.Config{
				PreGeneratedStreamIDs: 1000,
				WriteBatchSize:        3,
				WriteParallelLimit:    6,
			})
			err = migrator.MigrateProjects(ctx)
			require.NoError(t, err)

			checkMigration(t, ctx, metabaseDB)
		})
	}
}

func createLastSegment(ctx context.Context, pointerDB metainfo.PointerDB, projectID uuid.UUID, bucket, encryptedKey []byte, numberOfSegments int) error {
	pointer := &pb.Pointer{}
	pointer.Type = pb.Pointer_REMOTE
	pointer.SegmentSize = 10
	pointer.CreationDate = time.Now()
	pointer.ExpirationDate = time.Now()
	pointer.Remote = &pb.RemoteSegment{
		RootPieceId:  testrand.PieceID(),
		Redundancy:   &pb.RedundancyScheme{},
		RemotePieces: []*pb.RemotePiece{},
	}

	streamMeta := &pb.StreamMeta{}
	streamMeta.NumberOfSegments = int64(numberOfSegments)
	streamMeta.EncryptedStreamInfo = testrand.Bytes(1024)
	streamMeta.EncryptionBlockSize = 256
	streamMeta.EncryptionType = int32(pb.CipherSuite_ENC_AESGCM)
	streamMeta.LastSegmentMeta = &pb.SegmentMeta{
		EncryptedKey: testrand.Bytes(256),
		KeyNonce:     testrand.Bytes(32),
	}
	metaBytes, err := pb.Marshal(streamMeta)
	if err != nil {
		return err
	}
	pointer.Metadata = metaBytes

	path := strings.Join([]string{projectID.String(), "l", string(bucket), string(encryptedKey)}, "/")

	pointerBytes, err := pb.Marshal(pointer)
	if err != nil {
		return err
	}
	err = pointerDB.Put(ctx, storage.Key(path), storage.Value(pointerBytes))
	if err != nil {
		return err
	}
	return nil
}

func createSegment(ctx context.Context, pointerDB metainfo.PointerDB, projectID uuid.UUID, segmentIndex uint32, bucket, encryptedKey []byte) error {
	pointer := &pb.Pointer{}
	pointer.Type = pb.Pointer_REMOTE
	pointer.SegmentSize = 10
	pointer.CreationDate = time.Now()
	pointer.ExpirationDate = time.Now()
	pointer.Remote = &pb.RemoteSegment{
		RootPieceId:  testrand.PieceID(),
		Redundancy:   &pb.RedundancyScheme{},
		RemotePieces: []*pb.RemotePiece{},
	}

	segmentMeta := &pb.SegmentMeta{
		EncryptedKey: testrand.Bytes(256),
		KeyNonce:     testrand.Bytes(32),
	}

	metaBytes, err := pb.Marshal(segmentMeta)
	if err != nil {
		return err
	}
	pointer.Metadata = metaBytes

	path := strings.Join([]string{projectID.String(), "s" + strconv.Itoa(int(segmentIndex)), string(bucket), string(encryptedKey)}, "/")

	pointerBytes, err := pb.Marshal(pointer)
	if err != nil {
		return err
	}
	err = pointerDB.Put(ctx, storage.Key(path), storage.Value(pointerBytes))
	if err != nil {
		return err
	}
	return nil
}
