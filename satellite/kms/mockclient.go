// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms

import (
	"context"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	"storj.io/common/storj"
)

var (
	// MockChecksumMismatch can be used as a key info version to signal to mockGsmClient to return a checksum that does not match.
	MockChecksumMismatch = "mock-checksum-mismatch"

	// MockAccessSecretVersionError can be used as a key info version to signal to mockGsmClient to return an error from AccessSecretVersion method.
	MockAccessSecretVersionError = "mock-access-secret-version-error"

	// MockKeyNotFound can be used as a key info version to signal to mockGsmClient to return no payload data from AccessSecretVersion.
	MockKeyNotFound = "mock-key-not-found"
)

type mockGsmClient struct {
	config Config
}

func (c *mockGsmClient) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (_ *secretmanagerpb.AccessSecretVersionResponse, err error) {
	key, err := storj.NewKey([]byte(req.Name))
	if err != nil {
		return nil, err
	}

	resp := &secretmanagerpb.AccessSecretVersionResponse{
		Payload: &secretmanagerpb.SecretPayload{},
	}
	if req.Name == MockKeyNotFound {
		return resp, nil
	}

	var checksum int64
	for _, ki := range c.config.KeyInfos.Values {
		if ki.SecretVersion == req.Name {
			if req.Name == MockChecksumMismatch {
				checksum = ki.SecretChecksum + 1
			} else {
				checksum = ki.SecretChecksum
			}
			break
		}
	}

	resp.Payload.Data = key[:]
	resp.Payload.DataCrc32C = &checksum

	return resp, nil
}

func (c *mockGsmClient) Close() error {
	return nil
}
