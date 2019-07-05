// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storj"
)

func newStore(key storj.Key) *Store {
	store := NewStore()
	if err := store.Add("bucket", paths.Unencrypted{}, paths.Encrypted{}, key); err != nil {
		panic(err)
	}
	return store
}

func TestStoreEncryption(t *testing.T) {
	forAllCiphers(func(cipher storj.CipherSuite) {
		for i, rawPath := range []string{
			"",
			"/",
			"//",
			"file.txt",
			"file.txt/",
			"fold1/file.txt",
			"fold1/fold2/file.txt",
			"/fold1/fold2/fold3/file.txt",
		} {
			errTag := fmt.Sprintf("test:%d path:%q cipher:%v", i, rawPath, cipher)

			store := newStore(testrand.Key())
			path := paths.NewUnencrypted(rawPath)

			encPath, err := EncryptPath("bucket", path, cipher, store)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			decPath, err := DecryptPath("bucket", encPath, cipher, store)
			if !assert.NoError(t, err, errTag) {
				continue
			}

			assert.Equal(t, rawPath, decPath.Raw(), errTag)
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

func TestSegmentEncoding(t *testing.T) {
	segments := [][]byte{
		{},
		{'a'},
		{0},
		{'/'},
		{'a', 'b', 'c', 'd', '1', '2', '3', '4', '5'},
		{'/', '/', '/', '/', '/'},
		{0, 0, 0, 0, 0, 0, 0, 0, 0},
		{'a', '/', 'a', '2', 'a', 'a', 0, '1', 'b', 255},
		{'/', '/', 'a', 0, 'a', 'a', 0, '1', 'b', 'g', 'a', 'b', '/'},
		{0, '/', 'a', '0', 'a', 'a', 0, '1', 'b', 'g', 'a', 'b', 0},
	}

	// additional random segment
	segments = append(segments, testrand.BytesInt(255))

	for _, segment := range segments {
		encoded := encodeSegment(segment)
		require.Equal(t, -1, bytes.IndexByte(encoded, 0))
		require.Equal(t, -1, bytes.IndexByte(encoded, 255))
		require.Equal(t, -1, bytes.IndexByte(encoded, '/'))

		decoded, err := decodeSegment(encoded)
		require.NoError(t, err)
		require.Equal(t, segment, decoded)
	}
}

func TestInvalidSegmentDecoding(t *testing.T) {
	encoded := []byte{3, 4, 5, 6, 7}
	// first byte should be '\x01' or '\x02'
	_, err := decodeSegment(encoded)
	require.Error(t, err)
}

func BenchmarkSegmentEncoding(b *testing.B) {
	segments := [][]byte{
		{},
		{'a'},
		{0},
		{'/'},
		{'a', 'b', 'c', 'd', '1', '2', '3', '4', '5'},

		{'/', '/', '/', '/', '/'},
		{0, 0, 0, 0, 0, 0, 0, 0, 0},

		{'a', '/', 'a', '2', 'a', 'a', 0, '1', 'b', 255},
		{'/', '/', 'a', 0, 'a', 'a', 0, '1', 'b', 'g', 'a', 'b', '/'},
		{0, '/', 'a', '0', 'a', 'a', 0, '1', 'b', 'g', 'a', 'b', 0},
	}

	b.Run("Loop", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, segment := range segments {
				encoded := encodeSegment(segment)
				_, _ = decodeSegment(encoded)
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
