// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/storage"
)

// RunQueueTests runs common storage.Queue tests
func RunQueueTests(t *testing.T, q storage.Queue) {
	t.Run("basic", func(t *testing.T) { testBasic(t, q) })
}

func testBasic(t *testing.T, q storage.Queue) {
	err := q.Enqueue(storage.Value("hello world"))
	assert.NoError(t, err)
	err = q.Enqueue(storage.Value("Привіт, світе"))
	assert.NoError(t, err)
	err = q.Enqueue(storage.Value([]byte{0, 0, 0, 0, 255, 255, 255, 255}))
	assert.NoError(t, err)
	list, err := q.Peekqueue()
	assert.Equal(t, storage.Value([]byte{0, 0, 0, 0, 255, 255, 255, 255}), list[0])
	assert.Equal(t, storage.Value("Привіт, світе"), list[1])
	assert.Equal(t, storage.Value("hello world"), list[2])
	assert.NoError(t, err)
	out, err := q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, out, storage.Value("hello world"))
	out, err = q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, out, storage.Value("Привіт, світе"))
	out, err = q.Dequeue()
	assert.NoError(t, err)
	assert.Equal(t, out, storage.Value([]byte{0, 0, 0, 0, 255, 255, 255, 255}))
	out, err = q.Dequeue()
	assert.Nil(t, out)
	assert.Error(t, err)
}
