// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

func TestLoadEncryptionKey(t *testing.T) {
	saveRawKey := func(key []byte) (filepath string, clenaup func()) {
		t.Helper()

		ctx := testcontext.New(t)
		filename := ctx.File("encryption.key")
		err := ioutil.WriteFile(filename, key, os.FileMode(0400))
		require.NoError(t, err)

		return filename, ctx.Cleanup
	}

	t.Run("ok", func(t *testing.T) {
		passphrase := make([]byte, rand.Intn(100)+1)
		_, err := rand.Read(passphrase)
		require.NoError(t, err)

		expectedKey, err := storj.NewKey(passphrase)
		require.NoError(t, err)
		filename, cleanup := saveRawKey(expectedKey[:])
		defer cleanup()

		key, err := uplink.LoadEncryptionKey(filename)
		require.NoError(t, err)
		require.Equal(t, expectedKey, key)
	})

	t.Run("error: empty filepath", func(t *testing.T) {
		_, err := uplink.LoadEncryptionKey("")
		require.Error(t, err)
	})

	t.Run("error: file not found", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		filename := ctx.File("encryption.key")

		_, err := uplink.LoadEncryptionKey(filename)
		require.Error(t, err)
	})
}

func TestUseOrLoadEncryptionKey(t *testing.T) {
	saveRawKey := func(key []byte) (filepath string, clenaup func()) {
		t.Helper()

		ctx := testcontext.New(t)
		filename := ctx.File("encryption.key")
		err := ioutil.WriteFile(filename, key, os.FileMode(0400))
		require.NoError(t, err)

		return filename, ctx.Cleanup
	}

	t.Run("ok: load", func(t *testing.T) {
		passphrase := make([]byte, rand.Intn(100)+1)
		_, err := rand.Read(passphrase)
		require.NoError(t, err)

		expectedKey, err := storj.NewKey(passphrase)
		require.NoError(t, err)
		filename, cleanup := saveRawKey(expectedKey[:])
		defer cleanup()

		key, err := uplink.UseOrLoadEncryptionKey("", filename)
		require.NoError(t, err)
		require.Equal(t, expectedKey, key)
	})

	t.Run("ok: use", func(t *testing.T) {
		rawKey := make([]byte, rand.Intn(100)+1)
		_, err := rand.Read(rawKey)
		require.NoError(t, err)

		key, err := uplink.UseOrLoadEncryptionKey(string(rawKey), "")
		require.NoError(t, err)
		require.Equal(t, rawKey[:storj.KeySize], key[:])
	})

	t.Run("error", func(t *testing.T) {
		_, err := uplink.UseOrLoadEncryptionKey("", "")
		require.Error(t, err)
	})
}
