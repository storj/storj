// Copyright (C) 2018 Storj Labs, Inc.
//
// This file is part of the Storj client library.
//
// The Storj client library is free software: you can redistribute it and/or
// modify it under the terms of the GNU Lesser General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// The Storj client library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with The Storj client library.  If not, see
// <http://www.gnu.org/licenses/>.

package client

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

const mockMnemonic = "uncle obtain april oxygen cover patient layer abuse off text royal normal"

func TestDecryptBucketName(t *testing.T) {
	for i, tt := range []struct {
		encryptedName string
		mnemonic      string
		decryptedName string
		errString     string
	}{
		{"", "", "", "Invalid mnemonic"},
		{"", mockMnemonic, "", "Invalid encrypted name"},
		{mockEncryptedBucketName, "", "", "Invalid mnemonic"},
		{mockEncryptedBucketName, mockMnemonic, mockDecryptedBucketName, ""},
		{mockEncryptedBucketNameDiffMnemonic, mockMnemonic, "", "cipher: message authentication failed"},
	} {
		decryptedName, err := decryptBucketName(tt.encryptedName, tt.mnemonic)
		errTag := fmt.Sprintf("Test case #%d", i)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, tt.decryptedName, decryptedName, errTag)
		}
	}
}
