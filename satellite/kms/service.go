// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/storj"
	"storj.io/common/sync2"
)

// Error is the default error class for the package.
var Error = errs.Class("kms")

// Config is a configuration struct for secret management Service.
type Config struct {
	SecretVersion  string `help:"version name of the master key in Google Secret Manager. E.g.: projects/{projectID}/secrets/{secretName}/versions/{latest}" default:""`
	SecretChecksum int64  `help:"checksum of the master key in Google Secret Manager" default:"0"`
	TestMasterKey  string `help:"a fake master key to be used for the purpose of testing" releaseDefault:"" devDefault:"test-master-key" hidden:"true"`
}

// Service is a service for encrypting/decrypting project passphrases.
//
// architecture: Service
type Service struct {
	secretsService SecretsService
	config         Config

	initialized sync2.Fence
}

// NewService creates a new Service.
func NewService(config Config) *Service {
	return &Service{
		config: config,
	}
}

// Initialize initializes the service.
func (s *Service) Initialize(ctx context.Context) (err error) {
	var secretService SecretsService
	if s.config.SecretVersion != "" {
		secretService = newGsmService(s.config)
	} else {
		secretService = newMockSecretService(s.config)
	}

	err = secretService.Initialize(ctx)
	if err != nil {
		return err
	}

	s.secretsService = secretService

	s.initialized.Release()

	return nil
}

// GenerateEncryptedPassphrase generates a cryptographically random passphrase,
// returning its encrypted form.
func (s *Service) GenerateEncryptedPassphrase(ctx context.Context) ([]byte, error) {
	randBytes := make([]byte, storj.KeySize)
	_, err := rand.Read(randBytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	passphrase := make([]byte, base64.StdEncoding.EncodedLen(storj.KeySize))
	base64.StdEncoding.Encode(passphrase, randBytes)

	return s.EncryptPassphrase(ctx, passphrase)
}

// EncryptPassphrase encrypts the provided passphrase using the masterKey in an
// XSalsa20 and Poly1305 encryption.
func (s *Service) EncryptPassphrase(ctx context.Context, passphrase []byte) ([]byte, error) {
	if !s.initialized.Wait(ctx) {
		return nil, Error.New("service not initialized")
	}

	var nonce storj.Nonce
	_, err := rand.Read(nonce[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}

	masterKey, err := s.secretsService.getMasterKey()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	cipherText, err := encryption.EncryptSecretBox(passphrase, masterKey, &nonce)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// Prepend the nonce to the encrypted passphrase.
	encryptedPassphrase := make([]byte, storj.NonceSize+len(cipherText))
	copy(encryptedPassphrase[:storj.NonceSize], nonce[:])
	copy(encryptedPassphrase[storj.NonceSize:], cipherText)

	return encryptedPassphrase, nil
}

// DecryptPassphrase decrypts the provided encrypted passphrase using
// the masterKey.
func (s *Service) DecryptPassphrase(ctx context.Context, encryptedPassphrase []byte) ([]byte, error) {
	if !s.initialized.Wait(ctx) {
		return nil, Error.New("service not initialized")
	}

	masterKey, err := s.secretsService.getMasterKey()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var nonce storj.Nonce
	copy(nonce[:], encryptedPassphrase[:storj.NonceSize])

	plaintext, err := encryption.DecryptSecretBox(encryptedPassphrase[storj.NonceSize:], masterKey, &nonce)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return plaintext, nil
}

// Close closes the service.
func (s *Service) Close() error {
	return s.secretsService.Close()
}
