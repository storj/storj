// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms

import (
	"context"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	"storj.io/common/storj"
)

// gsmService is a service for encrypting/decrypting project passphrases.
// it uses Google Secret Manager as the master key provider.
type gsmService struct {
	client client
}

// newGsmService creates new gsmService for encrypting/decrypting project passphrases.
// this will get the master keys from Google Secret Manager and validate them.
func newGsmService(ctx context.Context, config Config) (*gsmService, error) {
	var client client
	if config.MockClient {
		client = &mockGsmClient{
			keyInfos: config.KeyInfos,
		}
	} else {
		internalClient, err := secretmanager.NewClient(ctx)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		client = &gsmClient{internalClient}
	}

	return &gsmService{
		client: client,
	}, nil
}

// GetKey gets and validates a key.
func (s *gsmService) GetKey(ctx context.Context, k KeyInfo) (*storj.Key, error) {
	keyRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: k.SecretVersion,
	}
	keyResp, err := s.client.AccessSecretVersion(ctx, keyRequest)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if keyResp.Payload.Data == nil || len(keyResp.Payload.Data) == 0 {
		return nil, Error.New("no key found in secret manager")
	}

	if k.SecretChecksum != keyResp.Payload.GetDataCrc32C() {
		return nil, Error.New("checksum mismatch")
	}

	return storj.NewKey(keyResp.Payload.Data)
}

// Close closes the service.
func (s *gsmService) Close() error {
	return s.client.Close()
}
