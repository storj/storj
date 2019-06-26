// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
)

func TestNewKey(t *testing.T) {
	t.Run("nil humanReadableKey", func(t *testing.T) {
		t.Parallel()

		key, err := storj.NewKey(nil)
		require.NoError(t, err)
		require.True(t, key.IsZero(), "key isn't zero value")
	})

	t.Run("empty humanReadableKey", func(t *testing.T) {
		t.Parallel()

		key, err := storj.NewKey([]byte{})
		require.NoError(t, err)
		require.True(t, key.IsZero(), "key isn't zero value")
	})

	t.Run("humanReadableKey is of KeySize length", func(t *testing.T) {
		t.Parallel()

		humanReadableKey := testrand.Bytes(storj.KeySize)

		key, err := storj.NewKey(humanReadableKey)
		require.NoError(t, err)
		require.Equal(t, humanReadableKey, key[:])
	})

	t.Run("humanReadableKey is shorter than KeySize", func(t *testing.T) {
		t.Parallel()

		humanReadableKey := testrand.BytesInt(testrand.Intn(storj.KeySize))

		key, err := storj.NewKey(humanReadableKey)
		require.NoError(t, err)
		require.Equal(t, humanReadableKey, key[:len(humanReadableKey)])
	})

	t.Run("humanReadableKey is larger than KeySize", func(t *testing.T) {
		t.Parallel()

		humanReadableKey := testrand.BytesInt(testrand.Intn(10) + storj.KeySize + 1)

		key, err := storj.NewKey(humanReadableKey)
		require.NoError(t, err)
		assert.Equal(t, humanReadableKey[:storj.KeySize], key[:])
	})

	t.Run("same human readable key produce the same key", func(t *testing.T) {
		t.Parallel()

		humanReadableKey := testrand.BytesInt(testrand.Intn(10) + storj.KeySize + 1)

		key1, err := storj.NewKey(humanReadableKey)
		require.NoError(t, err)

		key2, err := storj.NewKey(humanReadableKey)
		require.NoError(t, err)

		assert.Equal(t, key1, key2, "keys are equal")
	})
}

func TestKey_IsZero(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var key *storj.Key
		require.True(t, key.IsZero())

		wrapperFn := func(key *storj.Key) bool {
			return key.IsZero()
		}
		require.True(t, wrapperFn(nil))
	})

	t.Run("zero", func(t *testing.T) {
		key := &storj.Key{}
		require.True(t, key.IsZero())
	})

	t.Run("no nil/zero", func(t *testing.T) {
		key := &storj.Key{'k'}
		require.False(t, key.IsZero())
	})
}
