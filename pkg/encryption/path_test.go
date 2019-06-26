// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testrand"
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

			key := testrand.Key()

			encrypted, err := EncryptPath(path, cipher, &key)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			decrypted, err := DecryptPath(encrypted, cipher, &key)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			assert.Equal(t, path, decrypted, errTag)
		}
	})
}

func TestDeriveKey(t *testing.T) {
	forAllCiphers(func(cipher storj.Cipher) {
		for i, tt := range []struct {
			path      storj.Path
			depth     int
			errString string
		}{
			{"fold1/fold2/fold3/file.txt", -1, "encryption error: negative depth"},
			{"fold1/fold2/fold3/file.txt", 0, ""},
			{"fold1/fold2/fold3/file.txt", 1, ""},
			{"fold1/fold2/fold3/file.txt", 2, ""},
			{"fold1/fold2/fold3/file.txt", 3, ""},
			{"fold1/fold2/fold3/file.txt", 4, ""},
			{"fold1/fold2/fold3/file.txt", 5, "encryption error: depth greater than path length"},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			key := testrand.Key()

			encrypted, err := EncryptPath(tt.path, cipher, &key)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			derivedKey, err := DerivePathKey(tt.path, &key, tt.depth)
			if tt.errString != "" {
				assert.EqualError(t, err, tt.errString, errTag)
				continue
			}
			if !assert.NoError(t, err, errTag) {
				continue
			}

			shared := storj.JoinPaths(storj.SplitPath(encrypted)[tt.depth:]...)
			decrypted, err := DecryptPath(shared, cipher, derivedKey)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			expected := storj.JoinPaths(storj.SplitPath(tt.path)[tt.depth:]...)
			assert.Equal(t, expected, decrypted, errTag)
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

func TestSegmentEncoding(t *testing.T) {
	segments := [][]byte{
		[]byte{},
		[]byte{'a'},
		[]byte{0},
		[]byte{'/'},
		[]byte{'a', 'b', 'c', 'd', '1', '2', '3', '4', '5'},

		[]byte{'/', '/', '/', '/', '/'},
		[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0},

		[]byte{'a', '/', 'a', '2', 'a', 'a', 0, '1', 'b', 255},
		[]byte{'/', '/', 'a', 0, 'a', 'a', 0, '1', 'b', 'g', 'a', 'b', '/'},
		[]byte{0, '/', 'a', '0', 'a', 'a', 0, '1', 'b', 'g', 'a', 'b', 0},
	}

	for _, segment := range segments {
		encoded := encodeSegment(segment)
		decoded := decodeSegment(encoded)
		require.Equal(t, segment, decoded)
	}
}

func BenchmarkSegmentEncoding(b *testing.B) {
	segments := [][]byte{
		[]byte{},
		[]byte{'a'},
		[]byte{0},
		[]byte{'/'},
		[]byte{'a', 'b', 'c', 'd', '1', '2', '3', '4', '5'},

		[]byte{'/', '/', '/', '/', '/'},
		[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0},

		[]byte{'a', '/', 'a', '2', 'a', 'a', 0, '1', 'b', 255},
		[]byte{'/', '/', 'a', 0, 'a', 'a', 0, '1', 'b', 'g', 'a', 'b', '/'},
		[]byte{0, '/', 'a', '0', 'a', 'a', 0, '1', 'b', 'g', 'a', 'b', 0},
	}
	b.Run("Loop", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, segment := range segments {
				encoded := encodeSegment(segment)
				_ = decodeSegment(encoded)
			}
		}
	})
	b.Run("Base64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, segment := range segments {
				encoded := base64.RawURLEncoding.EncodeToString(segment)
				_, _ = base64.RawURLEncoding.DecodeString(encoded)
			}
		}
	})
}
