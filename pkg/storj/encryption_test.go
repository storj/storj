// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"storj.io/storj/pkg/storj"
)

func TestNewKey(t *testing.T) {
	t.Run("nil passphrase", func(t *testing.T) {
		t.Parallel()

		key, err := storj.NewKey(nil)
		require.NoError(t, err)
		require.False(t, key.IsZero(), "key is zero value")
	})

	t.Run("empty passphrase", func(t *testing.T) {
		t.Parallel()

		key, err := storj.NewKey([]byte{})
		require.NoError(t, err)
		require.False(t, key.IsZero(), "key is zero value")
	})

	t.Run("passphrase length less than or equal KeySize", func(t *testing.T) {
		t.Parallel()

		passphrase := make([]byte, rand.Intn(storj.KeySize)+1)
		_, err := rand.Read(passphrase)
		require.NoError(t, err)

		key, err := storj.NewKey(passphrase)
		require.NoError(t, err)
		require.False(t, key.IsZero(), "key is zero value")
		require.NotEqual(t, passphrase, key[:len(passphrase)])
	})

	t.Run("passphrase length greater than KeySize", func(t *testing.T) {
		t.Parallel()

		passphrase := make([]byte, rand.Intn(10)+storj.KeySize+1)
		_, err := rand.Read(passphrase)
		require.NoError(t, err)

		key, err := storj.NewKey(passphrase)
		require.NoError(t, err)
		require.False(t, key.IsZero(), "key is zero value")
		assert.NotEqual(t, passphrase[:storj.KeySize], key[:])
	})

	t.Run("same passphrase different key", func(t *testing.T) {
		t.Parallel()

		passphrase := make([]byte, rand.Intn(storj.KeySize)+1)
		_, err := rand.Read(passphrase)
		require.NoError(t, err)

		key1, err := storj.NewKey(passphrase)
		require.NoError(t, err)

		key2, err := storj.NewKey(passphrase)
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2, "keys are equal")
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
