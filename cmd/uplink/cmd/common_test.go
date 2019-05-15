// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/storj"
)

func TestLoadEncryptionKeyIntoEncryptionAccess(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		passphrase := make([]byte, rand.Intn(100)+1)
		_, err := rand.Read(passphrase)
		require.NoError(t, err)

		expectedKey, err := storj.NewKey(passphrase)
		require.NoError(t, err)
		ctx := testcontext.New(t)
		filename := ctx.File("encryption.key")
		err = ioutil.WriteFile(filename, expectedKey[:], os.FileMode(0400))
		require.NoError(t, err)
		defer ctx.Cleanup()

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

func TestUsOrLoadEncryptionKeyIntoEncryptionAccess(t *testing.T) {
	t.Run("ok: load", func(t *testing.T) {
		passphrase := make([]byte, rand.Intn(100)+1)
		_, err := rand.Read(passphrase)
		require.NoError(t, err)

		expectedKey, err := storj.NewKey(passphrase)
		require.NoError(t, err)
		ctx := testcontext.New(t)
		filename := ctx.File("encryption.key")
		err = ioutil.WriteFile(filename, expectedKey[:], os.FileMode(0400))
		require.NoError(t, err)
		defer ctx.Cleanup()

		access, err := useOrLoadEncryptionAccess("", filename)
		require.NoError(t, err)
		require.Equal(t, *expectedKey, access.Key)
	})

	t.Run("ok: use", func(t *testing.T) {
		rawKey := make([]byte, rand.Intn(100)+1)
		_, err := rand.Read(rawKey)
		require.NoError(t, err)

		access, err := useOrLoadEncryptionAccess(string(rawKey), "")
		require.NoError(t, err)
		require.Equal(t, rawKey[:storj.KeySize], access.Key[:])
	})

	t.Run("error", func(t *testing.T) {
		_, err := useOrLoadEncryptionAccess("", "")
		require.Error(t, err)
	})
}
