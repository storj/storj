// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/storj"
	"storj.io/common/sync2"
)

var (
	// Error is the default error class for the package.
	Error = errs.Class("kms")

	mon = monkit.Package()
)

// Service is a service for encrypting/decrypting project passphrases.
//
// architecture: Service
type Service struct {
	config Config

	defaultKey *storj.Key
	keys       map[int]*storj.Key

	initialized sync2.Fence
}

// NewService creates a new Service.
func NewService(config Config) *Service {
	return &Service{
		config: config,
		keys:   make(map[int]*storj.Key),
	}
}

// Run runs the service.
// NOTE: Run is automatically called by mud framework, but Initialize doesn't.
func (s *Service) Run(ctx context.Context) (err error) {
	return s.Initialize(ctx)
}

// Initialize initializes the service.
func (s *Service) Initialize(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var secretsService SecretsService
	switch s.config.Provider {
	case "gsm":
		secretsService, err = newGsmService(ctx, s.config)
		if err != nil {
			return err
		}
	case "local":
		secretsService = newLocalFileService(s.config)
	default:
		return Error.New("invalid encryption key provider: '%s'. See description of --kms.provider for supported values.", s.config.Provider)
	}

	defer func() {
		err = errs.Combine(err, secretsService.Close())
	}()

	s.keys, err = secretsService.GetKeys(ctx)
	if err != nil {
		return Error.New("error getting keys: %w", err)
	}

	key := s.keys[s.config.DefaultMasterKey]
	if key == nil {
		return Error.New("master key not set")
	}

	s.defaultKey = key

	s.initialized.Release()

	return nil
}

// GenerateEncryptedPassphrase generates a cryptographically random passphrase,
// returning its encrypted form and the id of the encryption key.
func (s *Service) GenerateEncryptedPassphrase(ctx context.Context) (_ []byte, keyID int, err error) {
	randBytes := make([]byte, storj.KeySize)
	_, err = rand.Read(randBytes)
	if err != nil {
		return nil, 0, Error.Wrap(err)
	}

	passphrase := make([]byte, base64.StdEncoding.EncodedLen(storj.KeySize))
	base64.StdEncoding.Encode(passphrase, randBytes)

	return s.EncryptPassphrase(ctx, passphrase)
}

// EncryptPassphrase encrypts the provided passphrase using the masterKey in an
// XSalsa20 and Poly1305 encryption.
func (s *Service) EncryptPassphrase(ctx context.Context, passphrase []byte) (_ []byte, keyID int, err error) {
	if !s.initialized.Wait(ctx) {
		return nil, 0, Error.New("service not initialized")
	}

	var nonce storj.Nonce
	_, err = rand.Read(nonce[:])
	if err != nil {
		return nil, 0, Error.Wrap(err)
	}

	cipherText, err := encryption.EncryptSecretBox(passphrase, s.defaultKey, &nonce)
	if err != nil {
		return nil, 0, Error.Wrap(err)
	}

	// Prepend the nonce to the encrypted passphrase.
	encryptedPassphrase := make([]byte, storj.NonceSize+len(cipherText))
	copy(encryptedPassphrase[:storj.NonceSize], nonce[:])
	copy(encryptedPassphrase[storj.NonceSize:], cipherText)

	return encryptedPassphrase, s.config.DefaultMasterKey, nil
}

// DecryptPassphrase decrypts the provided encrypted passphrase using
// the masterKey.
func (s *Service) DecryptPassphrase(ctx context.Context, keyID int, encryptedPassphrase []byte) ([]byte, error) {
	if !s.initialized.Wait(ctx) {
		return nil, Error.New("service not initialized")
	}

	key := s.keys[keyID]
	if key == nil {
		return nil, Error.New("key with ID %d not found", keyID)
	}
	var nonce storj.Nonce
	copy(nonce[:], encryptedPassphrase[:storj.NonceSize])

	plaintext, err := encryption.DecryptSecretBox(encryptedPassphrase[storj.NonceSize:], key, &nonce)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return plaintext, nil
}
