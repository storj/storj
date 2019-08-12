// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storagenode/pieces"
)

func BenchmarkReadWrite(b *testing.B) {
	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	dir, err := filestore.NewDir(ctx.Dir("pieces"))
	require.NoError(b, err)
	blobs := filestore.New(dir)
	defer ctx.Check(blobs.Close)

	store := pieces.NewStore(zap.NewNop(), blobs)

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

			require.NoError(b, writer.Commit(ctx))
		}
	})

	testPieceID := storj.PieceID{1}
	{ // write a test piece
		writer, err := store.Writer(ctx, satelliteID, testPieceID)
		require.NoError(b, err)
		_, err = writer.Write(source)
		require.NoError(b, err)
		require.NoError(b, writer.Commit(ctx))
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
