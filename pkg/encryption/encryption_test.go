// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/eestream"
)

const (
	uint32Size = 4
)

func TestCalcEncryptedSize(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	forAllCiphers(func(cipher storj.CipherSuite) {
		for i, dataSize := range []int64{
			0,
			1,
			1*memory.KiB.Int64() - uint32Size,
			1 * memory.KiB.Int64(),
			32*memory.KiB.Int64() - uint32Size,
			32 * memory.KiB.Int64(),
			32*memory.KiB.Int64() + 100,
		} {
			errTag := fmt.Sprintf("%d-%d. %+v", cipher, i, dataSize)

			parameters := storj.EncryptionParameters{CipherSuite: cipher, BlockSize: 1 * memory.KiB.Int32()}

			calculatedSize, err := encryption.CalcEncryptedSize(dataSize, parameters)
			require.NoError(t, err, errTag)

			encrypter, err := encryption.NewEncrypter(parameters.CipherSuite, new(storj.Key), new(storj.Nonce), int(parameters.BlockSize))
			require.NoError(t, err, errTag)

			randReader := ioutil.NopCloser(io.LimitReader(testrand.Reader(), dataSize))
			reader := encryption.TransformReader(eestream.PadReader(randReader, encrypter.InBlockSize()), encrypter, 0)

			cipherData, err := ioutil.ReadAll(reader)
			assert.NoError(t, err, errTag)
			assert.EqualValues(t, calculatedSize, len(cipherData), errTag)
		}
	})
}

func forAllCiphers(test func(cipher storj.CipherSuite)) {
	for _, cipher := range []storj.CipherSuite{
		storj.EncNull,
		storj.EncAESGCM,
		storj.EncSecretBox,
	} {
		test(cipher)
	}
}
