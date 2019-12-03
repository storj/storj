// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"storj.io/storj/private/testcontext"
)

func BenchmarkTransformReader(b *testing.B) {
	b.StopTimer()
	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	// current encryption blocksize
	trans := &testTransformer{blockSize: 7424 - 16, overhead: make([]byte, 16)}

	data := make([]byte, trans.blockSize*1415) // block-aligned, roughly 10MB
	_, err := rand.Read(data)
	require.NoError(b, err)

	source, err := os.Create(ctx.File("source"))
	require.NoError(b, err)
	_, err = source.Write(data)
	require.NoError(b, err)
	err = source.Close()
	require.NoError(b, err)

	source, err = os.Open(ctx.File("source"))
	require.NoError(b, err)
	defer func() {
		require.NoError(b, source.Close())
	}()

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, err = source.Seek(0, os.SEEK_SET)
		require.NoError(b, err)
		dest, err := os.Create(ctx.File(fmt.Sprintf("dest-%d", n)))
		require.NoError(b, err)
		defer func() {
			require.NoError(b, dest.Close())
		}()
		_, err = io.Copy(dest, TransformReader(source, trans, 0))
		require.NoError(b, err)
	}
	b.StopTimer()
}

type testTransformer struct {
	blockSize int
	overhead  []byte
}

func (t *testTransformer) InBlockSize() int  { return t.blockSize }
func (t *testTransformer) OutBlockSize() int { return t.blockSize + len(t.overhead) }

func (t *testTransformer) Transform(out, in []byte, blockNum int64) ([]byte, error) {
	return append(append(out, in...), t.overhead...), nil
}
