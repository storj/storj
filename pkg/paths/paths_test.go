// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package paths

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathEncryption(t *testing.T) {
	for i, path := range [][]string{
		[]string{},   // empty path
		[]string{""}, // empty path segment
		[]string{"file.txt"},
		[]string{"fold1", "file.txt"},
		[]string{"fold1", "fold2", "file.txt"},
		[]string{"fold1", "fold2", "fold3", "file.txt"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		key := []byte("my secret")
		encryptedPath, err := Encrypt(path, key)
		if !assert.NoError(t, err, errTag) {
			return
		}
		decryptedPath, err := Decrypt(encryptedPath, key)
		if !assert.NoError(t, err, errTag) {
			return
		}
		assert.Equal(t, path, decryptedPath, errTag)
	}
}

func TestDeriveKey(t *testing.T) {
	for i, tt := range []struct {
		path       []string
		shareLevel int
	}{
		{[]string{"fold1", "fold2", "fold3", "file.txt"}, 0},
		{[]string{"fold1", "fold2", "fold3", "file.txt"}, 1},
		{[]string{"fold1", "fold2", "fold3", "file.txt"}, 2},
		{[]string{"fold1", "fold2", "fold3", "file.txt"}, 3},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		key := []byte("my secret")
		encryptedPath, err := Encrypt(tt.path, key)
		if !assert.NoError(t, err, errTag) {
			return
		}
		sharedPath := encryptedPath[tt.shareLevel:]
		derivedKey := DeriveKey(key, tt.path[:tt.shareLevel])
		decryptedPath, err := Decrypt(sharedPath, derivedKey)
		if !assert.NoError(t, err, errTag) {
			return
		}
		assert.Equal(t, tt.path[tt.shareLevel:], decryptedPath, errTag)
	}
}
