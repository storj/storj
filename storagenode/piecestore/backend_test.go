// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/mwc"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/retain"
)

func BenchmarkPieceStore(b *testing.B) {
	var satellite storj.NodeID

	run := func(b *testing.B, backendFunc func(b *testing.B) PieceBackend, size int64) {
		backend := backendFunc(b)
		if cl, ok := backend.(interface{ Close() }); ok {
			defer cl.Close()
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

			wr, err := backend.Writer(context.Background(), satellite, piece, pb.PieceHashAlgorithm_BLAKE3, time.Time{})
			require.NoError(b, err)
			_, err = wr.Write(buf)
			require.NoError(b, err)
			require.NoError(b, wr.Commit(context.Background(), &pb.PieceHeader{
				Hash: wr.Hash(),
			}))
		}

		b.ReportMetric(float64(b.N)/time.Since(now).Seconds(), "pieces/sec")
	}

	b.Run("HashStore", func(b *testing.B) {
		run(b, func(b *testing.B) PieceBackend {
			bfm, _ := retain.NewBloomFilterManager(b.TempDir())
			rtm := retain.NewRestoreTimeManager(b.TempDir())
			backend := NewHashStoreBackend(b.TempDir(), bfm, rtm, nil)
			return backend
		}, 64*1024)
	})
}
