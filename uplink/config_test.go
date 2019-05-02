// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/storj"
)

func TestEncryptionConfig_LoadKey(t *testing.T) {
	saveKey := func(key []byte) (filepath string, clenaup func()) {
		t.Helper()

		ctx := testcontext.New(t)
		filename := ctx.File("encryption.key")
		err := ioutil.WriteFile(filename, key, os.FileMode(0400))
		require.NoError(t, err)

		return filename, ctx.Cleanup
	}

	t.Run("ok: file with key length less or equal than max size", func(t *testing.T) {
		someKey := make([]byte, rand.Intn(20)+1)
		_, err := rand.Read(someKey)
		require.NoError(t, err)
		filename, cleanup := saveKey(someKey)
		defer cleanup()

		var expectedKey storj.Key
		copy(expectedKey[:], someKey)

		encCfg := &EncryptionConfig{
			KeyFilepath: filename,
		}
		key, err := encCfg.LoadKey()
		require.NoError(t, err)

		assert.Equal(t, expectedKey[:], key[:])
	})

	t.Run("ok: file with key length greater than max size", func(t *testing.T) {
		expectedKey := make([]byte, rand.Intn(10)+1+storj.KeySize)
		_, err := rand.Read(expectedKey)
		require.NoError(t, err)
		filename, cleanup := saveKey(expectedKey)
		defer cleanup()

		encCfg := &EncryptionConfig{
			KeyFilepath: filename,
		}
		key, err := encCfg.LoadKey()
		require.NoError(t, err)

		assert.Equal(t, expectedKey[:storj.KeySize], key[:])
	})

	t.Run("ok: empty file", func(t *testing.T) {
		filename, cleanup := saveKey([]byte{})
		defer cleanup()

		encCfg := &EncryptionConfig{
			KeyFilepath: filename,
		}
		key, err := encCfg.LoadKey()
		require.NoError(t, err)
		assert.Equal(t, key, storj.Key{})
	})

	t.Run("error: KeyFilepath is empty", func(t *testing.T) {
		encCfg := &EncryptionConfig{}

		_, err := encCfg.LoadKey()
		require.Error(t, err)
		assert.True(t, Error.Has(err), "err is not of %q class", Error)
		assert.Equal(t, ErrKeyFilepathEmpty, err, "unexpected error value")
	})

	t.Run("error: file not found", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		filename := ctx.File("encryption.key")

		encCfg := &EncryptionConfig{
			KeyFilepath: filename,
		}
		_, err := encCfg.LoadKey()
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("not found key file %q", filename))
		assert.True(t, Error.Has(err), "err is not of %q class", Error)
	})
}

func TestEncryptionConfig_SaveKey(t *testing.T) {
	var expectedKey *storj.Key
	{
		key := make([]byte, rand.Intn(20)+1)
		_, err := rand.Read(key)
		require.NoError(t, err)
		expectedKey = storj.NewKey(key)
	}

	t.Run("ok", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		filename := ctx.File("storj-test-cmd-uplink", "encryption.key")
		encryptionCfg := &EncryptionConfig{
			KeyFilepath: filename,
		}
		err := encryptionCfg.SaveKey(expectedKey)
		require.NoError(t, err)

		key, err := ioutil.ReadFile(filename)
		require.NoError(t, err)
		assert.Equal(t, expectedKey, storj.NewKey(key))
	})

	t.Run("error: unexisting dir", func(t *testing.T) {
		// Create a directory and remove it for making sure that the path doesn't
		// exist
		ctx := testcontext.New(t)
		dir := ctx.Dir("storj-test-cmd-uplink")
		ctx.Cleanup()

		encryptionCfg := &EncryptionConfig{
			KeyFilepath: filepath.Join(dir, "enc.key"),
		}
		err := encryptionCfg.SaveKey(expectedKey)
		require.Errorf(t, err, "directory path doesn't exist")
	})

	t.Run("error: file already exists", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		filename := ctx.File("encryption.key")
		require.NoError(t, ioutil.WriteFile(filename, nil, os.FileMode(0600)))

		encryptionCfg := &EncryptionConfig{
			KeyFilepath: filename,
		}
		err := encryptionCfg.SaveKey(expectedKey)
		require.Errorf(t, err, "file key already exists")
	})
}
