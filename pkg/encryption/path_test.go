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

	// additional random segments
	for i := 0; i < 20; i++ {
		segments = append(segments, testrand.BytesInt(testrand.Intn(256)))
	}

	for i, segment := range segments {
		encoded := encodeSegment(segment)
		require.Equal(t, -1, bytes.IndexByte(encoded, 0))
		require.Equal(t, -1, bytes.IndexByte(encoded, 255))
		require.Equal(t, -1, bytes.IndexByte(encoded, '/'))

		decoded, err := decodeSegment(encoded)
		require.NoError(t, err, "#%d", i)
		require.Equal(t, segment, decoded, "#%d", i)
	}
}

func TestInvalidSegmentDecoding(t *testing.T) {
	encoded := []byte{3, 4, 5, 6, 7}
	// first byte should be '\x01' or '\x02'
	_, err := decodeSegment(encoded)
	require.Error(t, err)
}

func TestValidateEncodedSegment(t *testing.T) {
	// all segments should be invalid
	encodedSegments := [][]byte{
		{},
		{1, 1},
		{2},
		{2, 0},
		{2, '\xff'},
		{2, '\x2f'},
		{2, escapeSlash, '3'},
		{2, escapeFF, '3'},
		{2, escape01, '3'},
		{3, 4, 4, 4},
	}

	for i, segment := range encodedSegments {
		_, err := decodeSegment(segment)
		require.Error(t, err, "#%d", i)
	}
}

func TestEncodingDecodingStress(t *testing.T) {
	allCombinations := func(emit func([]byte)) {
		length := 3
		s := make([]byte, length)
		last := length - 1
		var combination func(int, int)
		combination = func(i int, next int) {
			for j := next; j < 256; j++ {
				s[i] = byte(j)
				if i == last {
					emit(s)
				} else {
					combination(i+1, j+1)
				}
			}
			return
		}
		combination(0, 0)
	}

	// all combinations for length 3
	allCombinations(func(segment []byte) {
		_ = encodeSegment(segment)
		_, _ = decodeSegment(segment)
	})

	// random segments
	for i := 0; i < 20; i++ {
		segment := testrand.BytesInt(testrand.Intn(256))
		_ = encodeSegment(segment)
		_, _ = decodeSegment(segment)
	}
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

	// additional random segment
	segments = append(segments, testrand.BytesInt(255))

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
