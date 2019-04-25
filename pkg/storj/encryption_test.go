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
		key := storj.NewKey(nil)
		require.Equal(t, &storj.Key{}, key)
	})

	t.Run("empty passphrase", func(t *testing.T) {
		key := storj.NewKey([]byte{})
		require.Equal(t, &storj.Key{}, key)
	})

	t.Run("passphrase length less than or equal KeySize", func(t *testing.T) {
		passphrase := make([]byte, rand.Intn(storj.KeySize)+1)
		_, err := rand.Read(passphrase)
		require.NoError(t, err)
		key := storj.NewKey(passphrase)
		assert.Equal(t, passphrase, key[:len(passphrase)])
		assert.Equal(t, make([]byte, storj.KeySize-len(passphrase)), key[len(passphrase):])
	})

	t.Run("passphrase length greater than KeySize", func(t *testing.T) {
		passphrase := make([]byte, rand.Intn(10)+storj.KeySize+1)
		_, err := rand.Read(passphrase)
		require.NoError(t, err)
		key := storj.NewKey(passphrase)
		assert.Equal(t, passphrase[:storj.KeySize], key[:])
	})
}
