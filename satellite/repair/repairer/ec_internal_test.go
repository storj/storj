// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPieceBufferClass(t *testing.T) {
	for _, tt := range []struct {
		size  int64
		class int
	}{
		{size: -1, class: -1},
		{size: 0, class: -1},
		{size: minPooledPieceBuffer - 1, class: -1},
		{size: minPooledPieceBuffer, class: 0},
		{size: minPooledPieceBuffer + 1, class: 1},
		{size: 2 * minPooledPieceBuffer, class: 1},
		{size: maxPooledPieceBuffer, class: len(pieceBufferPools) - 1},
		{size: maxPooledPieceBuffer + 1, class: -1},
	} {
		require.Equal(t, tt.class, pieceBufferClass(tt.size), "size %d", tt.size)
	}
}

func TestGetPieceBuffer(t *testing.T) {
	// a pooled buffer rounds its capacity up to its class, an unpooled one is
	// exactly the size asked for.
	for _, tt := range []struct {
		size int64
		cap  int
	}{
		{size: 1, cap: 1},
		{size: minPooledPieceBuffer, cap: minPooledPieceBuffer},
		{size: minPooledPieceBuffer + 1, cap: 2 * minPooledPieceBuffer},
		{size: maxPooledPieceBuffer, cap: maxPooledPieceBuffer},
		{size: maxPooledPieceBuffer + 1, cap: maxPooledPieceBuffer + 1},
	} {
		buffer := getPieceBuffer(tt.size)
		require.Len(t, *buffer, int(tt.size), "size %d", tt.size)
		require.Equal(t, tt.cap, cap(*buffer), "size %d", tt.size)

		// being handed back a buffer that was never pooled has to be tolerated.
		putPieceBuffer(buffer)
	}
}

// TestPooledPieceReader covers the buffer reuse: a recycled buffer still holds
// the previous piece, so a reader must not expose anything past the piece it
// was given, and must stop reading out of the buffer once it is closed.
func TestPooledPieceReader(t *testing.T) {
	buffer := getPieceBuffer(minPooledPieceBuffer)
	for i := range *buffer {
		(*buffer)[i] = 0xff
	}
	*buffer = (*buffer)[:copy(*buffer, "piece")]

	reader := newPooledPieceReader(buffer)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, []byte("piece"), data)

	require.NoError(t, reader.Close())
	require.NoError(t, reader.Close()) // safe more than once

	data, err = io.ReadAll(reader)
	require.NoError(t, err)
	require.Empty(t, data)
}

// TestPooledPieceReaderConcurrentClose covers the handoff to the decoder: it
// closes the piece readers it was handed without waiting for the goroutines
// reading them to stop, so reads overlap the close that recycles the buffer.
func TestPooledPieceReaderConcurrentClose(t *testing.T) {
	for range 100 {
		buffer := getPieceBuffer(minPooledPieceBuffer)
		reader := newPooledPieceReader(buffer)

		var group sync.WaitGroup
		start := make(chan struct{})
		for range 4 {
			group.Add(1)
			go func() {
				defer group.Done()
				<-start
				_, _ = io.Copy(io.Discard, reader)
			}()
		}
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_ = reader.Close()
		}()

		close(start)
		group.Wait()
	}
}
