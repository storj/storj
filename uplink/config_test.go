// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
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

	t.Run("ok: reading from file", func(t *testing.T) {
		passphrase := testrand.BytesInt(1 + testrand.Intn(100))

		expectedKey, err := storj.NewKey(passphrase)
		require.NoError(t, err)
		filename, cleanup := saveRawKey(expectedKey[:])
		defer cleanup()

		key, err := uplink.LoadEncryptionKey(filename)
		require.NoError(t, err)
		require.Equal(t, expectedKey, key)
	})

	t.Run("ok: empty filepath", func(t *testing.T) {
		key, err := uplink.LoadEncryptionKey("")
		require.NoError(t, err)
		require.Equal(t, &storj.Key{}, key)
	})

	t.Run("error: file not found", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		filename := ctx.File("encryption.key")

		_, err := uplink.LoadEncryptionKey(filename)
		require.Error(t, err)
	})
}
