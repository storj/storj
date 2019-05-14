// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"storj.io/storj/pkg/pkcrypto"
)

func TestGenerateSalt(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		size := rand.Intn(100) + 8

		salt, err := pkcrypto.GenerateSalt(uint32(size))
		require.NoError(t, err)
		require.Len(t, salt, size)
		require.False(t, bytes.Equal(salt, make([]byte, size)))
	})

	t.Run("error: size less than 8", func(t *testing.T) {
		t.Parallel()
		size := rand.Intn(8)

		_, err := pkcrypto.GenerateSalt(uint32(size))
		require.Error(t, err)
		require.True(t, pkcrypto.ErrSalt.Has(err), "err isn't of ErrSalt class")
	})
}
