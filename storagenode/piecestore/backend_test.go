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

func TestHashStoreBackend_SpaceUsage(t *testing.T) {
	ctx := testcontext.New(t)

	// Create a hashstore backend with specific compaction settings
	config := hashstore.CreateDefaultConfig(hashstore.TableKind_HashTbl, false)
	config.Compaction.RewriteMultiple = 2.0 // Set specific rewrite multiple for predictable testing

	bfm, _ := retain.NewBloomFilterManager(t.TempDir(), 0)
	rtm := retain.NewRestoreTimeManager(t.TempDir())
	backend, err := NewHashStoreBackend(ctx, config, t.TempDir(), "", bfm, rtm, nil, nil)
	require.NoError(t, err)
	defer ctx.Check(backend.Close)

	satellite := storj.NodeID{1, 2, 3}

	// Write several pieces to create measurable space usage
	for i := 0; i < 5; i++ {
		piece := storj.PieceID{byte(i)}
		wr, err := backend.Writer(ctx, satellite, piece, pb.PieceHashAlgorithm_BLAKE3, time.Time{})
		require.NoError(t, err)

		data := make([]byte, 1024)
		for j := range data {
			data[j] = byte(i)
		}
		_, err = wr.Write(data)
		require.NoError(t, err)
		require.NoError(t, wr.Commit(ctx, &pb.PieceHeader{
			Hash: wr.Hash(),
		}))
	}

	// Get space usage
	spaceUsage := backend.SpaceUsage()

	// Verify Reserved field is populated
	require.NotZero(t, spaceUsage.Reserved, "Reserved space should be non-zero after writing pieces")

	// Verify other fields are populated as expected
	require.NotZero(t, spaceUsage.UsedForMetadata, "UsedForMetadata should be non-zero")

	// Reserved should equal the TableSize (one per store in the backend)
	// Since we only have one satellite, we should have data in one DB
	db := backend.dbs[satellite]
	require.NotNil(t, db)

	_, s0Stats, s1Stats := db.Stats()

	// Reserved should be the max of FreeRequired from both stores
	// since only one store can compact at a time
	expectedReserved := int64(max(s0Stats.FreeRequired, s1Stats.FreeRequired))
	require.Equal(t, expectedReserved, spaceUsage.Reserved)

	// UsedForMetadata should match the table sizes (not FreeRequired)
	expectedMetadata := int64(s0Stats.Table.TableSize + s1Stats.Table.TableSize)
	require.Equal(t, expectedMetadata, spaceUsage.UsedForMetadata)

	// Test with multiple satellites to ensure aggregation works correctly
	satellite2 := storj.NodeID{4, 5, 6}
	piece2 := storj.PieceID{10}
	wr2, err := backend.Writer(ctx, satellite2, piece2, pb.PieceHashAlgorithm_BLAKE3, time.Time{})
	require.NoError(t, err)
	_, err = wr2.Write(make([]byte, 512))
	require.NoError(t, err)
	require.NoError(t, wr2.Commit(ctx, &pb.PieceHeader{
		Hash: wr2.Hash(),
	}))

	// Get updated space usage
	spaceUsage2 := backend.SpaceUsage()

	// Reserved should have increased due to the second satellite's data
	require.Greater(t, spaceUsage2.Reserved, spaceUsage.Reserved, "Reserved should increase with more satellites")
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
