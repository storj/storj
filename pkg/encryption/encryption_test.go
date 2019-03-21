// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption_test

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/storj"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
)

const (
	uint32Size = 4
)

func TestCalcEncryptedSize(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	forAllCiphers(func(cipher storj.Cipher) {
		for i, dataSize := range []int64{
			0,
			1,
			1*memory.KiB.Int64() - uint32Size,
			1 * memory.KiB.Int64(),
			32 * memory.KiB.Int64(),
			32*memory.KiB.Int64() + 100,
		} {
			errTag := fmt.Sprintf("%d-%d. %+v", cipher, i, dataSize)

			scheme := storj.EncryptionScheme{Cipher: cipher, BlockSize: 1 * memory.KiB.Int32()}

			calculatedSize, err := encryption.CalcEncryptedSize(int64(dataSize), scheme)
			require.NoError(t, err, errTag)

			encrypter, err := encryption.NewEncrypter(scheme.Cipher, new(storj.Key), new(storj.Nonce), int(scheme.BlockSize))
			require.NoError(t, err, errTag)

			reader := encryption.TransformReader(eestream.PadReader(ioutil.NopCloser(io.LimitReader(rand.Reader, dataSize)), encrypter.InBlockSize()), encrypter, 0)

			cipherData, err := ioutil.ReadAll(reader)
			assert.NoError(t, err, errTag)
			assert.EqualValues(t, calculatedSize, len(cipherData), errTag)
		}
	})
}

func forAllCiphers(test func(cipher storj.Cipher)) {
	for _, cipher := range []storj.Cipher{
		storj.Unencrypted,
		storj.AESGCM,
		storj.SecretBox,
	} {
		test(cipher)
	}
}
