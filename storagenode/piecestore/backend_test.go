// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/mwc"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode/hashstore"
	"storj.io/storj/storagenode/retain"
)

func TestHashstoreBackendTrash(t *testing.T) {
	ctx := testcontext.New(t)

	// allocate a hash backend
	bfm, _ := retain.NewBloomFilterManager(t.TempDir(), 0)
	rtm := retain.NewRestoreTimeManager(t.TempDir())
	backend, err := NewHashStoreBackend(ctx, hashstore.CreateDefaultConfig(hashstore.TableKind_HashTbl, false), t.TempDir(), "", bfm, rtm, nil, nil)
	require.NoError(t, err)
	defer ctx.Check(backend.Close)

	// write an empty piece
	wr, err := backend.Writer(ctx, storj.NodeID{}, storj.PieceID{}, pb.PieceHashAlgorithm_BLAKE3, time.Time{})
	require.NoError(t, err)
	require.NoError(t, wr.Commit(ctx, &pb.PieceHeader{
		Hash: wr.Hash(),
	}))

	// set the restore time to way in the past and add an empty bloom filter way in the future that
	// will cause the piece to be trashed
	require.NoError(t, rtm.TestingSetRestoreTime(ctx, storj.NodeID{}, time.Now().AddDate(-1, 0, 0)))
	filter := bloomfilter.NewOptimal(1000, 0.01)
	require.NoError(t, bfm.Queue(ctx, storj.NodeID{}, &pb.RetainRequest{
		CreationDate: time.Now().AddDate(1, 0, 0),
		Filter:       filter.Bytes(),
	}))

	// compact to trigger the piece being flagged as trash
	require.NoError(t, backend.dbs[storj.NodeID{}].Compact(ctx))

	// ensure the piece is trash
	rd, err := backend.Reader(ctx, storj.NodeID{}, storj.PieceID{})
	require.NoError(t, err)
	defer ctx.Check(rd.Close)
	require.True(t, rd.Trash())
}

func TestPieceValid(t *testing.T) {
	ctx := testcontext.New(t)

	backend, err := NewHashStoreBackend(ctx, hashstore.CreateDefaultConfig(hashstore.TableKind_HashTbl, false), t.TempDir(), "", nil, nil, nil, nil)
	require.NoError(t, err)
	defer ctx.Check(backend.Close)

	var satellite storj.NodeID
	_, _ = mwc.Rand().Read(satellite[:])
	var pieceID storj.PieceID
	_, _ = mwc.Rand().Read(pieceID[:])

	// write a piece with like 1024 bytes of data
	wr, err := backend.Writer(ctx, satellite, pieceID, pb.PieceHashAlgorithm_BLAKE3, time.Time{})
	require.NoError(t, err)
	data := make([]byte, 1024)
	_, _ = mwc.Rand().Read(data)
	_, err = wr.Write(data)
	require.NoError(t, err)
	require.NoError(t, wr.Commit(ctx, &pb.PieceHeader{
		OrderLimit:    pb.OrderLimit{PieceId: pieceID},
		HashAlgorithm: pb.PieceHashAlgorithm_BLAKE3,
		Hash:          wr.Hash(),
	}))

	// read back the piece data directly from the db so that we get the full contents
	r, err := backend.dbs[satellite].Read(ctx, pieceID)
	require.NoError(t, err)
	defer ctx.Check(r.Close)

	contents, err := io.ReadAll(r)
	require.NoError(t, err)

	// verify that the pieceValid function agrees that the data is valid
	require.True(t, pieceValid(pieceID, contents))

	// check that any byte modification of the data portion causes pieceValid to return false
	for i := range 1024 {
		original := contents[i]
		contents[i] ^= 0xFF
		require.False(t, pieceValid(pieceID, contents), "modification at byte %d not detected", i)
		contents[i] = original
	}

	// check that any truncation causes pieceValid to return false
	for l := range contents {
		truncated := contents[:l]
		require.False(t, pieceValid(pieceID, truncated), "truncation to length %d not detected", l)
	}
}

func BenchmarkPieceStore(b *testing.B) {
	var satellite storj.NodeID

	run := func(b *testing.B, backendFunc func(b *testing.B) PieceBackend, size int64) {
		backend := backendFunc(b)
		if cl, ok := backend.(interface{ Close() error }); ok {
			defer func() { _ = cl.Close() }()
		}

		buf := make([]byte, size)
		_, _ = mwc.Rand().Read(buf)

		b.SetBytes(size)
		b.ReportAllocs()
		b.ResetTimer()

		now := time.Now()

		for i := 0; i < b.N; i++ {
			var piece storj.PieceID
			_, _ = mwc.Rand().Read(piece[:])

			wr, err := backend.Writer(b.Context(), satellite, piece, pb.PieceHashAlgorithm_BLAKE3, time.Time{})
			require.NoError(b, err)
			_, err = wr.Write(buf)
			require.NoError(b, err)
			require.NoError(b, wr.Commit(b.Context(), &pb.PieceHeader{
				Hash: wr.Hash(),
			}))
		}

		b.ReportMetric(float64(b.N)/time.Since(now).Seconds(), "pieces/sec")
	}

	b.Run("HashStore", func(b *testing.B) {
		run(b, func(b *testing.B) PieceBackend {
			bfm, _ := retain.NewBloomFilterManager(b.TempDir(), 0)
			rtm := retain.NewRestoreTimeManager(b.TempDir())
			backend, err := NewHashStoreBackend(b.Context(), hashstore.CreateDefaultConfig(hashstore.TableKind_HashTbl, false), b.TempDir(), "", bfm, rtm, nil, nil)
			require.NoError(b, err)
			return backend
		}, 64*1024)
	})
}
