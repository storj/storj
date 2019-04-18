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

func TestKeyFilepath_Key(t *testing.T) {
	saveKey := func(key []byte) (filepath string, removeFile func()) {
		f, err := ioutil.TempFile("", "storj-test-uplink-keyfilepath-*")
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		_, err = f.Write(key)
		require.NoError(t, err)

		err = f.Chmod(os.FileMode(0400))
		require.NoError(t, err)

		return f.Name(), func() { _ = os.Remove(f.Name()) }
	}

	t.Run("ok: file with key length less or equal than max size", func(t *testing.T) {
		someKey := make([]byte, rand.Intn(20)+1)
		_, err := rand.Read(someKey)
		require.NoError(t, err)
		fpath, cleanup := saveKey(someKey)
		defer cleanup()

		var expKey storj.Key
		copy(expKey[:], someKey)

		kfpath := keyFilepath(fpath)
		key, err := kfpath.Key()
		require.NoError(t, err)

		assert.Equal(t, expKey[:], key[:])
	})

	t.Run("ok: file with key length greater than max size", func(t *testing.T) {
		expKey := make([]byte, rand.Intn(10)+1+storj.KeySize)
		_, err := rand.Read(expKey)
		fpath, cleanup := saveKey(expKey)
		defer cleanup()

		kfpath := keyFilepath(fpath)
		key, err := kfpath.Key()
		require.NoError(t, err)

		assert.Equal(t, expKey[:storj.KeySize], key[:])
	})

	t.Run("ok: empty file path", func(t *testing.T) {
		fpath, cleanup := saveKey([]byte{})
		defer cleanup()

		kfpath := keyFilepath(fpath)
		key, err := kfpath.Key()
		require.NoError(t, err)
		assert.Equal(t, key, storj.Key{})
	})

	t.Run("error: file not found", func(t *testing.T) {
		// Create a temp file and delete it, to get a filepath which doesn't exist.
		f, err := ioutil.TempFile("", "storj-test-uplink-keyfilepath-*")
		require.NoError(t, err)
		err = f.Close()
		require.NoError(t, err)
		err = os.Remove(f.Name())
		require.NoError(t, err)

		kfpath := keyFilepath(f.Name())
		_, err = kfpath.Key()
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("not found key file %q", f.Name()))
		assert.True(t, Error.Has(err), "err is not of %q class", Error)
	})

	t.Run("error: permissions are too open", func(t *testing.T) {
		// Create a key file and change its permission for not being able to read it
		f, err := ioutil.TempFile("", "storj-test-uplink-keyfilepath-*")
		require.NoError(t, err)
		defer func() {
			_ = f.Close()
			_ = os.Remove(f.Name())
		}()

		err = f.Chmod(0401)
		require.NoError(t, err)

		kfpath := keyFilepath(f.Name())
		_, err = kfpath.Key()
		require.Error(t, err)
		assert.Contains(t,
			err.Error(), fmt.Sprintf("permissions '0401' for key file %q are too open", f.Name()),
		)
		assert.True(t, Error.Has(err), "err is not of %q class", Error)
	})
}
