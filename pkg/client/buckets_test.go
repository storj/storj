// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

const (
	testEncryptedBucketName             = "cqiNhd3Y16uXRBpRKbcGdrhVvouLRFlBM5O1jMUOr6OKJUVGpvv0LLaBv+6kqzyVvp5jFw=="
	testEncryptedBucketNameDiffMnemonic = "Yq3Ky6jJ7dwWiC9MEcb5nhAl5P0xfYe6jCwwwlzd1a1kZxKYLcft/WkOC8dhcwLb3Ka9xA=="
	testDecryptedBucketName             = "test"
)

func TestGetBuckets(t *testing.T) {
	var tsResponse string
	router := httprouter.New()
	router.GET("/buckets", basicAuth(func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		fmt.Fprintf(w, tsResponse)
	}, testBridgeUser, testBridgePass))
	ts := httptest.NewServer(router)
	defer ts.Close()

	for i, tt := range []struct {
		env       Env
		response  string
		buckets   []Bucket
		errString string
	}{
		{NewNoAuthTestEnv(ts), "", nil, "unexpected status code: 401"},
		{NewBadPassTestEnv(ts), "", nil, "unexpected status code: 401"},
		{NewTestEnv(ts), "", []Bucket{}, "unexpected end of JSON input"},
		{NewTestEnv(ts), "{", []Bucket{}, "unexpected end of JSON input"},
		{NewTestEnv(ts), "{}", []Bucket{}, "json: cannot unmarshal object into Go value of type []client.Bucket"},
		{NewTestEnv(ts), "[]", []Bucket{}, ""},
		{NewTestEnv(ts),
			`[
			  {
			    "id": "e3eca45f4d294132c07b49f4",
			    "name": "cqiNhd3Y16uXRBpRKbcGdrhVvouLRFlBM5O1jMUOr6OKJUVGpvv0LLaBv+6kqzyVvp5jFw==",
			    "created": "2016-10-12T14:40:21.259Z"
			  }
			]`, []Bucket{
				Bucket{
					ID:        "e3eca45f4d294132c07b49f4",
					Name:      testDecryptedBucketName,
					Created:   "2016-10-12T14:40:21.259Z",
					Decrypted: true,
				},
			}, ""},
		{NewNoMnemonicTestEnv(ts),
			`[
			  {
			    "id": "e3eca45f4d294132c07b49f4",
			    "name": "cqiNhd3Y16uXRBpRKbcGdrhVvouLRFlBM5O1jMUOr6OKJUVGpvv0LLaBv+6kqzyVvp5jFw==",
			    "created": "2016-10-12T14:40:21.259Z"
			  }
			]`, []Bucket{
				Bucket{
					ID:        "e3eca45f4d294132c07b49f4",
					Name:      testEncryptedBucketName,
					Created:   "2016-10-12T14:40:21.259Z",
					Decrypted: false,
				},
			}, ""},
		{NewTestEnv(ts),
			`[
			  {
			    "id": "e3eca45f4d294132c07b49f4",
			    "name": "Yq3Ky6jJ7dwWiC9MEcb5nhAl5P0xfYe6jCwwwlzd1a1kZxKYLcft/WkOC8dhcwLb3Ka9xA==",
			    "created": "2016-10-12T14:40:21.259Z"
			  }
			]`, []Bucket{
				Bucket{
					ID:        "e3eca45f4d294132c07b49f4",
					Name:      testEncryptedBucketNameDiffMnemonic,
					Created:   "2016-10-12T14:40:21.259Z",
					Decrypted: false,
				},
			}, ""},
		{NewTestEnv(ts),
			`[
			  {
			    "id": "e3eca45f4d294132c07b49f4",
			    "name": "test",
			    "created": "2016-10-12T14:40:21.259Z"
			  }
			]`, []Bucket{
				Bucket{
					ID:        "e3eca45f4d294132c07b49f4",
					Name:      testDecryptedBucketName,
					Created:   "2016-10-12T14:40:21.259Z",
					Decrypted: false,
				},
			}, ""},
		{NewNoMnemonicTestEnv(ts),
			`[
			  {
			    "id": "e3eca45f4d294132c07b49f4",
			    "name": "test",
			    "created": "2016-10-12T14:40:21.259Z"
			  }
			]`, []Bucket{
				Bucket{
					ID:        "e3eca45f4d294132c07b49f4",
					Name:      testDecryptedBucketName,
					Created:   "2016-10-12T14:40:21.259Z",
					Decrypted: false,
				},
			}, ""},
	} {
		tsResponse = tt.response
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
