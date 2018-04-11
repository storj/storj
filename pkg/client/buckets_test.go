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

const (
	mockEncryptedBucketName             = "cqiNhd3Y16uXRBpRKbcGdrhVvouLRFlBM5O1jMUOr6OKJUVGpvv0LLaBv+6kqzyVvp5jFw=="
	mockEncryptedBucketNameDiffMnemonic = "Yq3Ky6jJ7dwWiC9MEcb5nhAl5P0xfYe6jCwwwlzd1a1kZxKYLcft/WkOC8dhcwLb3Ka9xA=="
	mockDecryptedBucketName             = "test"
)

func TestGetBuckets(t *testing.T) {
	for i, tt := range []struct {
		env       Env
		response  string
		buckets   []Bucket
		errString string
	}{
		{NewMockNoAuthEnv(), "", nil, "Unexpected response code: 401"},
		{NewMockBadPassEnv(), "", nil, "Unexpected response code: 401"},
		{NewMockEnv(), "[]", []Bucket{}, ""},
		{NewMockEnv(),
			`[
			  {
			    "id": "e3eca45f4d294132c07b49f4",
			    "name": "cqiNhd3Y16uXRBpRKbcGdrhVvouLRFlBM5O1jMUOr6OKJUVGpvv0LLaBv+6kqzyVvp5jFw==",
			    "created": "2016-10-12T14:40:21.259Z"
			  }
			]`, []Bucket{
				Bucket{
					ID:        "e3eca45f4d294132c07b49f4",
					Name:      mockDecryptedBucketName,
					Created:   "2016-10-12T14:40:21.259Z",
					Decrypted: true,
				},
			}, ""},
		{NewMockNoMnemonicEnv(),
			`[
			  {
			    "id": "e3eca45f4d294132c07b49f4",
			    "name": "cqiNhd3Y16uXRBpRKbcGdrhVvouLRFlBM5O1jMUOr6OKJUVGpvv0LLaBv+6kqzyVvp5jFw==",
			    "created": "2016-10-12T14:40:21.259Z"
			  }
			]`, []Bucket{
				Bucket{
					ID:        "e3eca45f4d294132c07b49f4",
					Name:      mockEncryptedBucketName,
					Created:   "2016-10-12T14:40:21.259Z",
					Decrypted: false,
				},
			}, ""},
		{NewMockEnv(),
			`[
			  {
			    "id": "e3eca45f4d294132c07b49f4",
			    "name": "Yq3Ky6jJ7dwWiC9MEcb5nhAl5P0xfYe6jCwwwlzd1a1kZxKYLcft/WkOC8dhcwLb3Ka9xA==",
			    "created": "2016-10-12T14:40:21.259Z"
			  }
			]`, []Bucket{
				Bucket{
					ID:        "e3eca45f4d294132c07b49f4",
					Name:      mockEncryptedBucketNameDiffMnemonic,
					Created:   "2016-10-12T14:40:21.259Z",
					Decrypted: false,
				},
			}, ""},
		{NewMockEnv(),
			`[
			  {
			    "id": "e3eca45f4d294132c07b49f4",
			    "name": "test",
			    "created": "2016-10-12T14:40:21.259Z"
			  }
			]`, []Bucket{
				Bucket{
					ID:        "e3eca45f4d294132c07b49f4",
					Name:      mockDecryptedBucketName,
					Created:   "2016-10-12T14:40:21.259Z",
					Decrypted: false,
				},
			}, ""},
		{NewMockNoMnemonicEnv(),
			`[
			  {
			    "id": "e3eca45f4d294132c07b49f4",
			    "name": "test",
			    "created": "2016-10-12T14:40:21.259Z"
			  }
			]`, []Bucket{
				Bucket{
					ID:        "e3eca45f4d294132c07b49f4",
					Name:      mockDecryptedBucketName,
					Created:   "2016-10-12T14:40:21.259Z",
					Decrypted: false,
				},
			}, ""},
	} {
		mockGetBucketsResponse = tt.response
		buckets, err := GetBuckets(tt.env)
		errTag := fmt.Sprintf("Test case #%d", i)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, tt.buckets, buckets, errTag)
		}
	}
}
