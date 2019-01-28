// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pstore

import (
	"bytes"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
)

func TestStore(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	store := NewStorage(ctx.Dir("example"))
	defer ctx.Check(store.Close)

	pieceID := strings.Repeat("AB01", 10)

	source := make([]byte, 8000)
	_, _ = rand.Read(source[:])

	{ // write data
		w, err := store.Writer(pieceID)
		require.NoError(t, err)

		n, err := io.Copy(w, bytes.NewReader(source))
		assert.Equal(t, n, int64(len(source)))
		assert.NoError(t, err)

		assert.NoError(t, w.Close())
	}

	{ // valid reads
		read := func(offset, length int64) []byte {
			reader, err := store.Reader(ctx, pieceID, offset, length)
			if assert.NoError(t, err) {
				data, err := ioutil.ReadAll(reader)
				assert.NoError(t, err)
				assert.NoError(t, reader.Close())
				return data
			}
			return nil
		}

		assert.Equal(t, source, read(0, -1))
		assert.Equal(t, source, read(0, 16000))

		assert.Equal(t, source[10:1010], read(10, 1000))
		assert.Equal(t, source[10:11], read(10, 1))
	}

	{ // invalid reads
		badread := func(offset, length int64) error {
			reader, err := store.Reader(ctx, pieceID, offset, length)
			if err == nil {
				assert.NoError(t, reader.Close())
			}
			return err
		}

		assert.Error(t, badread(-100, 0))
		assert.Error(t, badread(-100, -10))
	}

	{ // test delete
		assert.NoError(t, store.Delete(pieceID))

		_, err := store.Reader(ctx, pieceID, 0, -1)
		assert.Error(t, err)
	}
}
