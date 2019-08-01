// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storagenode/pieces"
)

func TestPieces(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dir, err := filestore.NewDir(ctx.Dir("pieces"))
	require.NoError(t, err)

	blobs := filestore.New(dir, zaptest.NewLogger(t))
	defer ctx.Check(blobs.Close)

	store := pieces.NewStore(zaptest.NewLogger(t), blobs, nil, nil)

	satelliteID := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion()).ID
	pieceID := storj.NewPieceID()

	source := testrand.Bytes(8000)

	{ // write data
		writer, err := store.Writer(ctx, satelliteID, pieceID)
		require.NoError(t, err)

		n, err := io.Copy(writer, bytes.NewReader(source))
		require.NoError(t, err)
		assert.Equal(t, len(source), int(n))
		assert.Equal(t, len(source), int(writer.Size()))

		// verify hash
		hash := pkcrypto.NewHash()
		_, _ = hash.Write(source)
		assert.Equal(t, hash.Sum(nil), writer.Hash())

		// commit
		require.NoError(t, writer.Commit(ctx, &pb.PieceHeader{}))
		// after commit we should be able to call cancel without an error
		require.NoError(t, writer.Cancel(ctx))
	}

	{ // valid reads
		read := func(offset, length int64) []byte {
			reader, err := store.Reader(ctx, satelliteID, pieceID)
			require.NoError(t, err)

			pos, err := reader.Seek(offset, io.SeekStart)
			require.NoError(t, err)
			require.Equal(t, offset, pos)

			data := make([]byte, length)
			n, err := io.ReadFull(reader, data)
			require.NoError(t, err)
			require.Equal(t, int(length), n)

			require.NoError(t, reader.Close())

			return data
		}

		require.Equal(t, source[10:11], read(10, 1))
		require.Equal(t, source[10:1010], read(10, 1000))
		require.Equal(t, source, read(0, int64(len(source))))
	}

	{ // reading ends with io.EOF
		reader, err := store.Reader(ctx, satelliteID, pieceID)
		require.NoError(t, err)

		data := make([]byte, 111)
		for {
			_, err := reader.Read(data)
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}
		}

		require.NoError(t, reader.Close())
	}

	{ // test delete
		assert.NoError(t, store.Delete(ctx, satelliteID, pieceID))
		// read should now fail
		_, err := store.Reader(ctx, satelliteID, pieceID)
		assert.Error(t, err)
	}

	{ // write cancel
		cancelledPieceID := storj.NewPieceID()
		writer, err := store.Writer(ctx, satelliteID, cancelledPieceID)
		require.NoError(t, err)

		n, err := io.Copy(writer, bytes.NewReader(source))
		require.NoError(t, err)
		assert.Equal(t, len(source), int(n))
		assert.Equal(t, len(source), int(writer.Size()))

		// cancel writing
		require.NoError(t, writer.Cancel(ctx))
		// commit should not fail
		require.Error(t, writer.Commit(ctx, &pb.PieceHeader{}))

		// read should fail
		_, err = store.Reader(ctx, satelliteID, cancelledPieceID)
		assert.Error(t, err)
	}
}

func writeAPiece(ctx context.Context, t testing.TB, store *pieces.Store, satelliteID storj.NodeID, pieceID storj.PieceID, data []byte, atTime time.Time, formatVersion storage.FormatVersion) {
	tStore := &pieces.StoreForTest{store}
	writer, err := tStore.WriterForFormatVersion(ctx, satelliteID, pieceID, formatVersion)
	require.NoError(t, err)

	_, err = writer.Write(data)
	require.NoError(t, err)
	size := writer.Size()
	assert.Equal(t, int64(len(data)), size)
	err = writer.Commit(ctx, &pb.PieceHeader{
		Hash:         writer.Hash(),
		CreationTime: atTime,
	})
	require.NoError(t, err)
}

func verifyPieceHandle(t testing.TB, reader *pieces.Reader, expectDataLen int, expectCreateTime time.Time, expectFormat storage.FormatVersion) {
	assert.Equal(t, expectFormat, reader.GetStorageFormatVersion())
	assert.Equal(t, int64(expectDataLen), reader.Size())
	if expectFormat != storage.FormatV0 {
		pieceHeader, err := reader.GetPieceHeader()
		require.NoError(t, err)
		assert.Equal(t, expectFormat, storage.FormatVersion(pieceHeader.FormatVersion))
		assert.Equal(t, expectCreateTime.UTC(), pieceHeader.CreationTime.UTC())
	}
}

func tryOpeningAPiece(ctx context.Context, t testing.TB, store *pieces.Store, satelliteID storj.NodeID, pieceID storj.PieceID, expectDataLen int, expectTime time.Time, expectFormat storage.FormatVersion) {
	reader, err := store.Reader(ctx, satelliteID, pieceID)
	require.NoError(t, err)
	verifyPieceHandle(t, reader, expectDataLen, expectTime, expectFormat)
	require.NoError(t, reader.Close())

	reader, err = store.ReaderSpecific(ctx, satelliteID, pieceID, expectFormat)
	require.NoError(t, err)
	verifyPieceHandle(t, reader, expectDataLen, expectTime, expectFormat)
	require.NoError(t, reader.Close())
}

// Test that the piece store can still read V0 pieces that might be left over from a previous
// version, as well as V1 pieces.
func TestMultipleStorageFormatVersions(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	blobs, err := filestore.NewAt(ctx.Dir("store"), zaptest.NewLogger(t))
	require.NoError(t, err)
	defer ctx.Check(blobs.Close)

	store := pieces.NewStore(zaptest.NewLogger(t), blobs, nil, nil)

	const pieceSize = 1024

	var (
		data      = testrand.Bytes(pieceSize)
		satellite = testrand.NodeID()
		v0PieceID = testrand.PieceID()
		v1PieceID = testrand.PieceID()
		now       = time.Now()
	)

	// write a V0 piece
	writeAPiece(ctx, t, store, satellite, v0PieceID, data, now, storage.FormatV0)

	// write a V1 piece
	writeAPiece(ctx, t, store, satellite, v1PieceID, data, now, storage.FormatV1)

	// look up the different pieces with Reader and ReaderSpecific
	tryOpeningAPiece(ctx, t, store, satellite, v0PieceID, len(data), now, storage.FormatV0)
	tryOpeningAPiece(ctx, t, store, satellite, v1PieceID, len(data), now, storage.FormatV1)

	// write a V1 piece with the same ID as the V0 piece (to simulate it being rewritten as
	// V1 during a migration)
	differentData := append(data, 111, 104, 97, 105)
	writeAPiece(ctx, t, store, satellite, v0PieceID, differentData, now, storage.FormatV1)

	// if we try to access the piece at that key, we should see only the V1 piece
	tryOpeningAPiece(ctx, t, store, satellite, v0PieceID, len(differentData), now, storage.FormatV1)

	// unless we ask specifically for a V0 piece
	reader, err := store.ReaderSpecific(ctx, satellite, v0PieceID, storage.FormatV0)
	require.NoError(t, err)
	verifyPieceHandle(t, reader, len(data), now, storage.FormatV0)
	require.NoError(t, reader.Close())

	// delete the v0PieceID; both the V0 and the V1 pieces should go away
	err = store.Delete(ctx, satellite, v0PieceID)
	require.NoError(t, err)

	reader, err = store.Reader(ctx, satellite, v0PieceID)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	assert.Nil(t, reader)
}
