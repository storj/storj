// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms

import (
	"context"

	"storj.io/common/storj"
)

// mockSecretService is a service for encrypting/decrypting project passphrases.
// it is intended to be used in tests.
type mockSecretService struct {
	config    Config
	masterKey *storj.Key
}

// newMockSecretService returns a mockSecretService.
func newMockSecretService(config Config) *mockSecretService {
	return &mockSecretService{
		config: config,
	}
}

// Initialize gets and validates the master key.
func (s *mockSecretService) Initialize(_ context.Context) (err error) {
	s.masterKey, err = storj.NewKey([]byte(s.config.TestMasterKey))
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// getMasterKey returns the master key.
func (s *mockSecretService) getMasterKey() (*storj.Key, error) {
	return s.masterKey, nil
}

// Close closes the service.
func (s *mockSecretService) Close() error {
	return nil
}
