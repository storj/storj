// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package setup_test

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
	"storj.io/storj/uplink/setup"
)

func TestLoadEncryptionAccess(t *testing.T) {
	saveRawCtx := func(access *libuplink.EncryptionAccess) (filepath string, clenaup func()) {
		t.Helper()

		ctx := testcontext.New(t)
		filename := ctx.File("encryption.ctx")
		data, err := access.Serialize()
		require.NoError(t, err)
		err = ioutil.WriteFile(filename, []byte(data), os.FileMode(0400))
		require.NoError(t, err)

		return filename, ctx.Cleanup
	}

	t.Run("ok: reading from file", func(t *testing.T) {
		passphrase := testrand.BytesInt(1 + testrand.Intn(100))

		key, err := storj.NewKey(passphrase)
		require.NoError(t, err)
		access := libuplink.NewEncryptionAccessWithDefaultKey(*key)
		filename, cleanup := saveRawCtx(access)
		defer cleanup()

		gotCtx, err := setup.LoadEncryptionAccess(context.Background(), uplink.EncryptionConfig{
			EncAccessFilepath: filename,
		})
		require.NoError(t, err)
		require.Equal(t, access, gotCtx)
	})

	t.Run("ok: empty filepath", func(t *testing.T) {
		gotCtx, err := setup.LoadEncryptionAccess(context.Background(), uplink.EncryptionConfig{
			EncAccessFilepath: "",
		})

		require.NoError(t, err)
		require.NotNil(t, gotCtx)
	})

	t.Run("error: file not found", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		filename := ctx.File("encryption.ctx")

		_, err := setup.LoadEncryptionAccess(context.Background(), uplink.EncryptionConfig{
			EncAccessFilepath: filename,
		})
		require.Error(t, err)
	})
}
