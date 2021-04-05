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
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage"
)

func TestMigrator_SingleSegmentObj(t *testing.T) {
	expectedEntries := 1
	expectedProjectID := testrand.UUID()
	expectedBucket := []byte("bucket-name")
	expectedObjectKey := []byte("encrypted-key")

	var pointer *pb.Pointer
	createPointers := func(t *testing.T, ctx context.Context, pointerDB metainfo.PointerDB) {
		var err error
		pointer, err = createLastSegment(ctx, pointerDB, expectedProjectID, expectedBucket, expectedObjectKey, 1)
		require.NoError(t, err)

		// create invalid segment key which should be ignored during migration
		err = pointerDB.Put(ctx, storage.Key("ff5b056b-5763-41f8-a928-286723cfefc9/l/test_bucket"), storage.Value([]byte{}))
		require.NoError(t, err)
	}

	checkMigration := func(t *testing.T, ctx context.Context, metabaseDB metainfo.MetabaseDB) {
		objects, err := metabaseDB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, expectedEntries)

		{ // verify object
			require.EqualValues(t, expectedProjectID, objects[0].ProjectID)
			require.EqualValues(t, expectedBucket, objects[0].BucketName)
			require.EqualValues(t, expectedObjectKey, objects[0].ObjectKey)
			require.EqualValues(t, pointer.SegmentSize, objects[0].TotalEncryptedSize)
			require.EqualValues(t, 1, objects[0].SegmentCount)
			require.Equal(t, metabase.Committed, objects[0].Status)
			require.Zero(t, objects[0].TotalPlainSize)
			require.WithinDuration(t, pointer.CreationDate, objects[0].CreatedAt, 5*time.Second)
			require.WithinDuration(t, pointer.ExpirationDate, *objects[0].ExpiresAt, 5*time.Second)

			streamMeta := &pb.StreamMeta{}
			err = pb.Unmarshal(pointer.Metadata, streamMeta)
			require.NoError(t, err)

			require.Equal(t, pointer.Metadata, objects[0].EncryptedMetadata)
			require.EqualValues(t, streamMeta.LastSegmentMeta.EncryptedKey, objects[0].EncryptedMetadataEncryptedKey)
			require.EqualValues(t, streamMeta.LastSegmentMeta.KeyNonce, objects[0].EncryptedMetadataNonce)

			require.EqualValues(t, streamMeta.EncryptionType, objects[0].Encryption.CipherSuite)
			require.EqualValues(t, streamMeta.EncryptionBlockSize, objects[0].Encryption.BlockSize)

		}

		{ // verify segment
			segments, err := metabaseDB.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Equal(t, len(segments), expectedEntries)

			require.Zero(t, segments[0].Position.Part)
			require.Zero(t, segments[0].Position.Index)
			require.Zero(t, segments[0].PlainOffset)
			require.Zero(t, segments[0].PlainSize)

			require.EqualValues(t, pointer.Remote.RootPieceId, segments[0].RootPieceID)

			redundancy := pointer.Remote.Redundancy
			require.EqualValues(t, redundancy.ErasureShareSize, segments[0].Redundancy.ShareSize)
			require.EqualValues(t, redundancy.Type, segments[0].Redundancy.Algorithm)
			require.EqualValues(t, redundancy.MinReq, segments[0].Redundancy.RequiredShares)
			require.EqualValues(t, redundancy.RepairThreshold, segments[0].Redundancy.RepairShares)
			require.EqualValues(t, redundancy.SuccessThreshold, segments[0].Redundancy.OptimalShares)
			require.EqualValues(t, redundancy.Total, segments[0].Redundancy.TotalShares)
			require.Empty(t, segments[0].InlineData)

			require.Equal(t, len(pointer.Remote.RemotePieces), len(segments[0].Pieces))
			for i, piece := range pointer.Remote.RemotePieces {
				require.EqualValues(t, piece.PieceNum, segments[0].Pieces[i].Number)
				require.Equal(t, piece.NodeId, segments[0].Pieces[i].StorageNode)
			}
		}
	}
	test(t, createPointers, checkMigration)
}

func TestMigrator_ManyOneSegObj(t *testing.T) {
	expectedEntries := 300
	createPointers := func(t *testing.T, ctx context.Context, pointerDB metainfo.PointerDB) {
		projectID := testrand.UUID()
		for i := 0; i < expectedEntries; i++ {
			_, err := createLastSegment(ctx, pointerDB, projectID, []byte("bucket-name"), []byte("encrypted-key"+strconv.Itoa(i)), 1)
			require.NoError(t, err)
		}

		// create invalid segment key which should be ignored during migration
		err := pointerDB.Put(ctx, storage.Key("005b056b-5763-41f8-a928-286723cfefc9/l/test_bucket"), storage.Value([]byte{}))
		require.NoError(t, err)
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

		_, err := createLastSegment(ctx, pointerDB, projectID, []byte("bucket-name"), []byte("encrypted-key"), expectedEntries+1)
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
	for _, satelliteDB := range satellitedbtest.Databases() {
		satelliteDB := satelliteDB
		if strings.EqualFold(satelliteDB.MasterDB.URL, "omit") {
			continue
		}
		t.Run(satelliteDB.Name, func(t *testing.T) {
			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			log := zaptest.NewLogger(t)

			schemaSuffix := satellitedbtest.SchemaSuffix()
			schema := satellitedbtest.SchemaName(t.Name(), "category", 0, schemaSuffix)

			pointerTempDB, err := tempdb.OpenUnique(ctx, satelliteDB.PointerDB.URL, schema)
			require.NoError(t, err)

			pointerDB, err := satellitedbtest.CreatePointerDBOnTopOf(ctx, log, pointerTempDB)
			require.NoError(t, err)
			defer ctx.Check(pointerDB.Close)

			schema = satellitedbtest.SchemaName(t.Name(), "category", 1, schemaSuffix)
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
				InvalidObjectsFile:    ctx.File(satelliteDB.Name + "_invalid_objects.csv"),
			})
			err = migrator.MigrateProjects(ctx)
			require.NoError(t, err)

			checkMigration(t, ctx, metabaseDB)
		})
	}
}

func createLastSegment(ctx context.Context, pointerDB metainfo.PointerDB, projectID uuid.UUID, bucket, encryptedKey []byte, numberOfSegments int) (*pb.Pointer, error) {
	pointer := &pb.Pointer{}
	pointer.Type = pb.Pointer_REMOTE
	pointer.SegmentSize = 10
	pointer.CreationDate = time.Now()
	pointer.ExpirationDate = time.Now()
	pointer.Remote = &pb.RemoteSegment{
		RootPieceId: testrand.PieceID(),
		Redundancy: &pb.RedundancyScheme{
			ErasureShareSize: 256,
			Type:             pb.RedundancyScheme_RS,

			MinReq:           1,
			RepairThreshold:  2,
			SuccessThreshold: 3,
			Total:            4,
		},
		RemotePieces: []*pb.RemotePiece{},
	}

	for i := 0; i < 10; i++ {
		pointer.Remote.RemotePieces = append(pointer.Remote.RemotePieces, &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   testrand.NodeID(),
		})
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
		return nil, err
	}
	pointer.Metadata = metaBytes

	path := strings.Join([]string{projectID.String(), "l", string(bucket), string(encryptedKey)}, "/")

	pointerBytes, err := pb.Marshal(pointer)
	if err != nil {
		return nil, err
	}
	err = pointerDB.Put(ctx, storage.Key(path), storage.Value(pointerBytes))
	if err != nil {
		return nil, err
	}
	return pointer, nil
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
