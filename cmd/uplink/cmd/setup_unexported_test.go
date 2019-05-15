// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
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

func TestSaveEncryptionKey(t *testing.T) {
	var expectedKey = &storj.Key{}
	{
		key := make([]byte, rand.Intn(20)+1)
		_, err := rand.Read(key)
		require.NoError(t, err)

		copy(expectedKey[:], key)
	}

	t.Run("ok", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		filename := ctx.File("storj-test-cmd-uplink", "encryption.key")
		err := saveEncryptionKey(expectedKey, filename)
		require.NoError(t, err)

		var key = &storj.Key{}
		{
			rawKey, err := ioutil.ReadFile(filename)
			require.NoError(t, err)
			copy(key[:], rawKey)
		}

		assert.Equal(t, expectedKey, key)
	})

	t.Run("error: unexisting dir", func(t *testing.T) {
		// Create a directory and remove it for making sure that the path doesn't
		// exist
		ctx := testcontext.New(t)
		dir := ctx.Dir("storj-test-cmd-uplink")
		ctx.Cleanup()

		filename := filepath.Join(dir, "enc.key")
		err := saveEncryptionKey(expectedKey, filename)
		require.Errorf(t, err, "directory path doesn't exist")
	})

	t.Run("error: file already exists", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		filename := ctx.File("encryption.key")
		require.NoError(t, ioutil.WriteFile(filename, nil, os.FileMode(0600)))

		err := saveEncryptionKey(expectedKey, filename)
		require.Errorf(t, err, "file key already exists")
	})
}
