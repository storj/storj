// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/storj"
)

func TestEncryption(t *testing.T) {
	forAllCiphers(func(cipher storj.Cipher) {
		for i, path := range []storj.Path{
			"",
			"/",
			"//",
			"file.txt",
			"file.txt/",
			"fold1/file.txt",
			"fold1/fold2/file.txt",
			"/fold1/fold2/fold3/file.txt",
		} {
			errTag := fmt.Sprintf("%d. %+v", i, path)

			key := new(storj.Key)
			copy(key[:], randData(storj.KeySize))

			encrypted, err := EncryptPath(path, cipher, key)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			decrypted, err := DecryptPath(encrypted, cipher, key)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			assert.Equal(t, path, decrypted, errTag)
		}
	})
}

func TestDeriveKey(t *testing.T) {
	forAllCiphers(func(cipher storj.Cipher) {
		for i, path := range [][2]storj.Path{
			{"fold1", "fold2"},
			{"fold1/fold2", "fold3"},
			{"fold1/fold2/fold3", "file.txt"},
		} {
			errTag := fmt.Sprintf("%d. %q", i, path)

			key := new(storj.Key)
			copy(key[:], randData(storj.KeySize))

			firstEncrypted, err := EncryptPath(path[0], cipher, key)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			derivedKey, err := DerivePathKey(path[0], key)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			secondEncrypted, err := EncryptPath(path[1], cipher, derivedKey)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			fullDerived := storj.JoinPaths(firstEncrypted, secondEncrypted)
			fullEncrypted, err := EncryptPath(storj.JoinPaths(path[:]...), cipher, key)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			assert.Equal(t, fullDerived, fullEncrypted, errTag)
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
