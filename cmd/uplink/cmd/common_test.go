// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

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

func TestLoadEncryptionKeyIntoEncryptionAccess(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		passphrase := testrand.BytesInt(testrand.Intn(100) + 1)

		expectedKey, err := storj.NewKey(passphrase)
		require.NoError(t, err)

		filename := ctx.File("encryption.key")
		err = ioutil.WriteFile(filename, expectedKey[:], os.FileMode(0400))
		require.NoError(t, err)

		access, err := loadEncryptionAccess(filename)
		require.NoError(t, err)
		require.Equal(t, *expectedKey, access.Key)
	})

	t.Run("error", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		filename := ctx.File("encryption.key")

		_, err := loadEncryptionAccess(filename)
		require.Error(t, err)
	})
}

func TestSaveLoadEncryptionKey(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	inputKey := string(testrand.BytesInt(testrand.Intn(storj.KeySize)*3 + 1))

	filename := ctx.File("storj-test-cmd-uplink", "encryption.key")
	err := uplink.SaveEncryptionKey(inputKey, filename)
	require.NoError(t, err)

	access, err := loadEncryptionAccess(filename)
	require.NoError(t, err)

	if len(inputKey) > storj.KeySize {
		require.Equal(t, []byte(inputKey[:storj.KeySize]), access.Key[:])
	} else {
		require.Equal(t, []byte(inputKey), access.Key[:len(inputKey)])
	}
}
