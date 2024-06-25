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
	client *secretmanager.Client

	config Config

	masterKey *storj.Key
}

// newGsmService creates new gsmService for encrypting/decrypting project passphrases.
// this will get the master key from Google Secret Manager and validate it.
func newGsmService(config Config) *gsmService {
	return &gsmService{
		config: config,
	}
}

// Initialize gets and validates the master key.
func (s *gsmService) Initialize(ctx context.Context) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	s.client = client

	keyRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: s.config.SecretVersion,
	}
	masterKey, err := s.client.AccessSecretVersion(ctx, keyRequest)
	if err != nil {
		return Error.Wrap(err)
	}

	if s.config.SecretChecksum != masterKey.Payload.GetDataCrc32C() {
		return Error.New("checksum mismatch")
	}

	if masterKey.Payload.Data == nil || len(masterKey.Payload.Data) == 0 {
		return Error.New("no master key found in secret manager")
	}

	s.masterKey, err = storj.NewKey(masterKey.Payload.Data)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// getMasterKey returns the master key.
func (s *gsmService) getMasterKey() (*storj.Key, error) {
	if s.masterKey == nil || len(s.masterKey) == 0 {
		return nil, Error.New("master key not initialized")
	}
	return s.masterKey, nil
}

// Close closes the service.
func (s *gsmService) Close() error {
	return s.client.Close()
}
