// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package statcache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
)

func TestSerialization(t *testing.T) {
	f := FileInfo{
		modTime: time.Now().Truncate(1 * time.Microsecond),
		size:    1234567,
	}
	bytes := serialize(f)
	after := deserialize(bytes)
	require.Equal(t, f.Size(), after.Size())
	require.Equal(t, f.ModTime(), after.ModTime())
}

func TestBadger(t *testing.T) {
	f := FileInfo{
		modTime: time.Now().Truncate(1 * time.Microsecond),
		size:    1234567,
	}

	cache, err := NewBadgerCache(zaptest.NewLogger(t), t.TempDir())
	require.NoError(t, err)

	ctx := testcontext.New(t)

	defer ctx.Check(cache.Close)

	err = cache.Set(ctx, []byte("ns"), []byte("key1"), f)
	require.NoError(t, err)

	after, found, err := cache.Get(ctx, []byte("ns"), []byte("key1"))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, f.Size(), after.Size())
	require.Equal(t, f.ModTime(), after.ModTime())

	_, found, err = cache.Get(ctx, []byte("ns"), []byte("key2"))
	require.NoError(t, err)
	require.False(t, found)

	err = cache.Delete(ctx, []byte("ns"), []byte("key1"))
	require.NoError(t, err)
	require.False(t, found)

	_, found, err = cache.Get(ctx, []byte("ns"), []byte("key1"))
	require.NoError(t, err)
	require.False(t, found)
}
