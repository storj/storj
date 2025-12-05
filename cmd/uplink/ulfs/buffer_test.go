// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/cmd/uplink/ulloc"
)

type testReadHandle struct {
	bytes.Reader
	info   ObjectInfo
	closed bool
}

func newTestReadHandle(content []byte, info ObjectInfo) *testReadHandle {
	return &testReadHandle{
		Reader: *bytes.NewReader(content),
		info:   info,
	}
}

func (rh *testReadHandle) Close() error {
	rh.closed = true
	return nil
}

func (rh *testReadHandle) Info() ObjectInfo {
	return rh.info
}

func TestBufferedReadHandle(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	size := 1 * memory.KiB
	content := testrand.Bytes(size)
	info := ObjectInfo{
		Loc:           ulloc.NewLocal("/test/path"),
		Created:       time.Now(),
		ContentLength: size.Int64(),
	}
	rh := newTestReadHandle(content, info)
	buf := make([]byte, size.Int())

	// Check that ObjectInfo is passed through correctly.
	bufrh := NewBufferedReadHandle(ctx, rh, buf)
	assert.Equal(t, info, bufrh.Info())

	// Byte slice for the read content.
	read := make([]byte, size.Int())

	// Read just one byte.
	n, err := bufrh.Read(read[:1])
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, content[0], read[0])

	// Check that the buffer has the content.
	assert.Equal(t, content, buf)

	// Read the rest.
	n, err = bufrh.Read(read[1:])
	require.NoError(t, err)
	assert.Equal(t, size.Int()-1, n)
	assert.Equal(t, content, read)

	// Reading more should return io.EOF.
	n, err = bufrh.Read(read)
	require.EqualError(t, err, io.EOF.Error())
	assert.Zero(t, n)

	// Check that Close closes the underlying reader.
	err = bufrh.Close()
	require.NoError(t, err)
	assert.True(t, rh.closed)
}

func TestBufferPool(t *testing.T) {
	// Create a pool with size 2
	bufSize := 1 * memory.KiB.Int()
	pool := NewBytesPool(bufSize)

	// Get one []bytes
	buf1 := pool.Get()
	require.Len(t, buf1, bufSize)

	// Write something to buf1.
	copy(buf1, "first")

	// Get second []byte.
	buf2 := pool.Get()
	require.Len(t, buf2, bufSize)

	// Write something to buf2.
	copy(buf2, "second")

	// The two []byte should be different.
	assert.NotEqual(t, buf1, buf2)

	// Put it back to the pool.
	pool.Put(buf2)

	// Get it back from the pool.
	buf3 := pool.Get()
	require.Len(t, buf3, bufSize)

	// Should be the same as buf2.
	assert.Equal(t, buf2, buf3)
}
