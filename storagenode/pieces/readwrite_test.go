// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storagenode/pieces"
)

func BenchmarkReadWrite(b *testing.B) {
	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	dir, err := filestore.NewDir(ctx.Dir("pieces"))
	require.NoError(b, err)
	blobs := filestore.New(zap.NewNop(), dir)
	defer ctx.Check(blobs.Close)

	store := pieces.NewStore(zap.NewNop(), blobs, nil, nil, nil)

	// setup test parameters
	const blockSize = int(256 * memory.KiB)
	satelliteID := testrand.NodeID()
	source := testrand.Bytes(30 * memory.MB)

	b.Run("Write", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pieceID := testrand.PieceID()
			writer, err := store.Writer(ctx, satelliteID, pieceID)
			require.NoError(b, err)

			data := source
			for len(data) > 0 {
				n := blockSize
				if n > len(data) {
					n = len(data)
				}
				_, err = writer.Write(data[:n])
				require.NoError(b, err)
				data = data[n:]
			}

			require.NoError(b, writer.Commit(ctx, &pb.PieceHeader{}))
		}
	})

	testPieceID := storj.PieceID{1}
	{ // write a test piece
		writer, err := store.Writer(ctx, satelliteID, testPieceID)
		require.NoError(b, err)
		_, err = writer.Write(source)
		require.NoError(b, err)
		require.NoError(b, writer.Commit(ctx, &pb.PieceHeader{}))
	}

	b.Run("Read", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			reader, err := store.Reader(ctx, satelliteID, testPieceID)
			require.NoError(b, err)

			data := make([]byte, blockSize)
			for {
				_, err := reader.Read(data)
				if err != nil {
					if err == io.EOF {
						break
					}
					require.NoError(b, err)
				}
			}
			require.NoError(b, reader.Close())
		}
	})
}

func readAndWritePiece(t *testing.T, content []byte) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dir, err := filestore.NewDir(ctx.Dir("pieces"))
	require.NoError(t, err)
	blobs := filestore.New(zaptest.NewLogger(t), dir)
	defer ctx.Check(blobs.Close)

	store := pieces.NewStore(zaptest.NewLogger(t), blobs, nil, nil, nil)

	// test parameters
	satelliteID := testrand.NodeID()
	pieceID := testrand.PieceID()
	fakeHash := testrand.Bytes(32)
	creationTime := time.Unix(1564362827, 18364029)
	fakeSig := testrand.Bytes(32)
	expirationTime := time.Unix(1595898827, 18364029)

	// write a V1 format piece
	w, err := store.Writer(ctx, satelliteID, pieceID)
	require.NoError(t, err)
	if len(content) > 0 {
		_, err = w.Write(content)
		require.NoError(t, err)
	}

	// make sure w.Size() works
	assert.Equal(t, int64(len(content)), w.Size())

	// commit the writer with the piece header, and close it
	err = w.Commit(ctx, &pb.PieceHeader{
		Hash:         fakeHash,
		CreationTime: creationTime,
		Signature:    fakeSig,
		OrderLimit: pb.OrderLimit{
			PieceExpiration: expirationTime.UTC(),
		},
	})
	require.NoError(t, err)

	// open a reader
	r, err := store.Reader(ctx, satelliteID, pieceID)
	require.NoError(t, err)
	defer ctx.Check(r.Close)
	assert.Equal(t, filestore.MaxFormatVersionSupported, r.StorageFormatVersion())

	// make sure r.Size() works
	assert.Equal(t, int64(len(content)), r.Size())

	// make sure seek-nowhere works as expected before piece header is read
	pos, err := r.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	require.Equal(t, int64(0), pos)

	// read piece header
	header, err := r.GetPieceHeader()
	require.NoError(t, err)
	assert.Equal(t, fakeHash, header.Hash)
	assert.Truef(t, header.CreationTime.Equal(creationTime),
		"header.CreationTime = %s, but expected creationTime = %s", header.CreationTime, creationTime)
	assert.Equal(t, fakeSig, header.Signature)
	require.NotZero(t, header.OrderLimit.PieceExpiration)
	assert.Truef(t, header.OrderLimit.PieceExpiration.Equal(expirationTime),
		"*header.ExpirationTime = %s, but expected expirationTime = %s", header.OrderLimit.PieceExpiration, expirationTime)
	assert.Equal(t, pb.OrderLimit{PieceExpiration: expirationTime.UTC()}, header.OrderLimit)
	assert.Equal(t, filestore.FormatV1, storage.FormatVersion(header.FormatVersion))

	// make sure seek-nowhere works as expected after piece header is read too
	// (from the point of view of the piece store, the file position has not moved)
	pos, err = r.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	assert.Equal(t, int64(0), pos)

	// read piece contents
	bufSize := memory.MB.Int()
	if len(content) < bufSize {
		bufSize = len(content)
	}
	buf := make([]byte, bufSize)
	bytesRead, err := r.Read(buf)
	require.NoError(t, err)
	require.Equal(t, bufSize, bytesRead)
	require.Equal(t, content[:len(buf)], buf)

	// GetPieceHeader should error here now
	header, err = r.GetPieceHeader()
	require.Error(t, err)
	assert.Truef(t, pieces.Error.Has(err), "err is not a pieces.Error: %v", err)
	assert.Nil(t, header)

	// check file position again
	pos, err = r.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	require.Equal(t, int64(bufSize), pos)

	const miniReadSize = 256
	if len(content) > int(pos+miniReadSize) {
		// Continuing to read should be ok
		bytesRead, err = r.Read(buf[:miniReadSize])
		require.NoError(t, err)
		require.Equal(t, miniReadSize, bytesRead)
		require.Equal(t, content[int(memory.MB):int(memory.MB)+miniReadSize], buf[:miniReadSize])

		// Perform a Seek that actually moves the file pointer
		const startReadFrom = 11
		pos, err = r.Seek(startReadFrom, io.SeekStart)
		require.NoError(t, err)
		assert.Equal(t, int64(startReadFrom), pos)

		// And make sure that Seek had an effect
		bytesRead, err = r.Read(buf[:miniReadSize])
		require.NoError(t, err)
		require.Equal(t, miniReadSize, bytesRead)
		require.Equal(t, content[startReadFrom:startReadFrom+miniReadSize], buf[:miniReadSize])
	}
}

func TestReadWriteWithPieceHeader(t *testing.T) {
	content := testrand.Bytes(30 * memory.MB)
	readAndWritePiece(t, content)
}

func TestEmptyPiece(t *testing.T) {
	var content [0]byte
	readAndWritePiece(t, content[:])
}
