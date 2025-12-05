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
	config Config
}

// newGsmService creates new gsmService for encrypting/decrypting project passphrases.
// this will get the master keys from Google Secret Manager and validate them.
func newGsmService(ctx context.Context, config Config) (*gsmService, error) {
	var client client
	if config.MockClient {
		client = &mockGsmClient{
			config: config,
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
		config: config,
	}, nil
}

// GetKeys gets keys from source.
func (s *gsmService) GetKeys(ctx context.Context) (keys map[int]*storj.Key, err error) {
	defer mon.Task()(&ctx)(&err)

	keys = make(map[int]*storj.Key)

	for id, k := range s.config.KeyInfos.Values {
		keyRequest := &secretmanagerpb.AccessSecretVersionRequest{
			Name: k.SecretVersion,
		}
		keyResp, err := s.client.AccessSecretVersion(ctx, keyRequest)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		if len(keyResp.Payload.Data) == 0 {
			return nil, Error.New("no key found in secret manager")
		}

		if k.SecretChecksum != keyResp.Payload.GetDataCrc32C() {
			return nil, Error.New("checksum mismatch")
		}

		keys[id], err = storj.NewKey(keyResp.Payload.Data)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	return keys, nil
}

// Close closes the service.
func (s *gsmService) Close() error {
	return s.client.Close()
}
