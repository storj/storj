// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const testMnemonic = "uncle obtain april oxygen cover patient layer abuse off text royal normal"

func TestDecryptBucketName(t *testing.T) {
	for i, tt := range []struct {
		encryptedName string
		mnemonic      string
		decryptedName string
		errString     string
	}{
		{"", "", "", "Invalid mnemonic"},
		{"", testMnemonic, "", "Invalid encrypted name"},
		{testEncryptedBucketName, "", "", "Invalid mnemonic"},
		{testEncryptedBucketName, testMnemonic, testDecryptedBucketName, ""},
		{testEncryptedBucketNameDiffMnemonic, testMnemonic, "", "cipher: message authentication failed"},
	} {
		decryptedName, err := decryptBucketName(tt.encryptedName, tt.mnemonic)
		if tt.errString != "" {
			assert.Containsf(t, err.Error(), tt.errString, "Test case #%d", i)
			continue
		}
		if assert.NoErrorf(t, err, "Test case #%d", i) {
			assert.Equalf(t, tt.decryptedName, decryptedName, "Test case #%d", i)
		}
	}
}
