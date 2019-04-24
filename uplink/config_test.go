// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"storj.io/storj/pkg/storj"
)

func TestEncryptionConfig_Key(t *testing.T) {
	saveKey := func(key []byte) (filepath string, removeFile func()) {
		t.Helper()

		file, err := ioutil.TempFile("", "storj-test-uplink-keyfilepath-*")
		require.NoError(t, err)
		defer func() { require.NoError(t, file.Close()) }()

		_, err = file.Write(key)
		require.NoError(t, err)

		err = file.Chmod(os.FileMode(0400))
		require.NoError(t, err)

		return file.Name(), func() { require.NoError(t, os.Remove(file.Name())) }
	}

	t.Run("ok: file with key length less or equal than max size", func(t *testing.T) {
		someKey := make([]byte, rand.Intn(20)+1)
		_, err := rand.Read(someKey)
		require.NoError(t, err)
		filename, cleanup := saveKey(someKey)
		defer cleanup()

		var expKey storj.Key
		copy(expKey[:], someKey)

		encCfg := &EncryptionConfig{
			KeyFilepath: filename,
		}
		key, err := encCfg.Key()
		require.NoError(t, err)

		assert.Equal(t, expKey[:], key[:])
	})

	t.Run("ok: file with key length greater than max size", func(t *testing.T) {
		expKey := make([]byte, rand.Intn(10)+1+storj.KeySize)
		_, err := rand.Read(expKey)
		require.NoError(t, err)
		filename, cleanup := saveKey(expKey)
		defer cleanup()

		encCfg := &EncryptionConfig{
			KeyFilepath: filename,
		}
		key, err := encCfg.Key()
		require.NoError(t, err)

		assert.Equal(t, expKey[:storj.KeySize], key[:])
	})

	t.Run("ok: empty file path", func(t *testing.T) {
		filename, cleanup := saveKey([]byte{})
		defer cleanup()

		encCfg := &EncryptionConfig{
			KeyFilepath: filename,
		}
		key, err := encCfg.Key()
		require.NoError(t, err)
		assert.Equal(t, key, storj.Key{})
	})

	t.Run("error: file not found", func(t *testing.T) {
		// Create a temp file and delete it, to get a filepath which doesn't exist.
		file, err := ioutil.TempFile("", "storj-test-uplink-keyfilepath-*")
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)
		err = os.Remove(file.Name())
		require.NoError(t, err)

		encCfg := &EncryptionConfig{
			KeyFilepath: file.Name(),
		}
		_, err = encCfg.Key()
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("not found key file %q", file.Name()))
		assert.True(t, Error.Has(err), "err is not of %q class", Error)
	})

	t.Run("error: permissions are too open", func(t *testing.T) {
		// Create a key file and change its permission for not being able to read it
		file, err := ioutil.TempFile("", "storj-test-uplink-keyfilepath-*")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, file.Close())
			require.NoError(t, os.Remove(file.Name()))
		}()

		err = file.Chmod(0401)
		require.NoError(t, err)

		encCfg := &EncryptionConfig{
			KeyFilepath: file.Name(),
		}
		_, err = encCfg.Key()
		require.Error(t, err)
		assert.Contains(t,
			err.Error(),
			fmt.Sprintf("permissions '0401' for key file %q are too open", file.Name()),
		)
		assert.True(t, Error.Has(err), "err is not of %q class", Error)
	})
}
