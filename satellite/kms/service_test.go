// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/kms"
)

func TestService(t *testing.T) {
	ctx := testcontext.New(t)

	key1 := 1
	key2 := 2
	t.Run("master key not set", func(t *testing.T) {
		var ki kms.KeyInfos
		require.NoError(t, ki.Set(fmt.Sprintf("%d:secretversion1,12345;%d:secretversion2,54321", key1, key2)))
		service := kms.NewService(kms.Config{
			MockClient:       true,
			KeyInfos:         ki,
			DefaultMasterKey: 3,
		})
		err := service.Initialize(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "master key not set")
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		var ki kms.KeyInfos
		require.NoError(t, ki.Set(fmt.Sprintf("%d:%s,12345", key1, kms.MockChecksumMismatch)))
		service := kms.NewService(kms.Config{
			MockClient:       true,
			KeyInfos:         ki,
			DefaultMasterKey: key1,
		})
		err := service.Initialize(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "checksum mismatch")
	})

	t.Run("key not found", func(t *testing.T) {
		var ki kms.KeyInfos
		require.NoError(t, ki.Set(fmt.Sprintf("%d:%s,12345", key1, kms.MockKeyNotFound)))
		service := kms.NewService(kms.Config{
			MockClient:       true,
			KeyInfos:         ki,
			DefaultMasterKey: key1,
		})
		err := service.Initialize(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no key found in secret manager")
	})

	t.Run("encrypt/decrypt passphrases", func(t *testing.T) {
		var ki kms.KeyInfos
		require.NoError(t, ki.Set(fmt.Sprintf("%d:secretversion1,12345;%d:secretversion2,54321", key1, key2)))
		service := kms.NewService(kms.Config{
			MockClient:       true,
			KeyInfos:         ki,
			DefaultMasterKey: key1,
		})

		err := service.Initialize(ctx)
		require.NoError(t, err)

		encryptedPassphrase, encKeyID, err := service.GenerateEncryptedPassphrase(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, encryptedPassphrase)
		require.Equal(t, key1, encKeyID)

		decryptedPassphrase, err := service.DecryptPassphrase(ctx, encKeyID, encryptedPassphrase)
		require.NoError(t, err)
		require.NotEmpty(t, decryptedPassphrase)

		// encrypting the decrypted passphrase should result in a different encrypted passphrase
		newEncryptedPassphrase, encKeyID, err := service.EncryptPassphrase(ctx, decryptedPassphrase)
		require.NoError(t, err)
		require.NotEqual(t, encryptedPassphrase, newEncryptedPassphrase)
		require.Equal(t, key1, encKeyID)

		newDecryptedPassphrase, err := service.DecryptPassphrase(ctx, encKeyID, newEncryptedPassphrase)
		require.NoError(t, err)
		require.Equal(t, decryptedPassphrase, newDecryptedPassphrase)

		// malformed encrypted passphrase should return an error
		wrongEncryptedPassphrase := encryptedPassphrase
		wrongEncryptedPassphrase = append(wrongEncryptedPassphrase, []byte("random")...)
		_, err = service.DecryptPassphrase(ctx, encKeyID, wrongEncryptedPassphrase)
		require.Error(t, err)

		wrongEncryptedPassphrase = encryptedPassphrase
		wrongEncryptedPassphrase = append([]byte("random"), wrongEncryptedPassphrase...)
		_, err = service.DecryptPassphrase(ctx, encKeyID, wrongEncryptedPassphrase)
		require.Error(t, err)

		// different master key should not be able to decrypt the passphrase.
		_, err = service.DecryptPassphrase(ctx, key2, encryptedPassphrase)
		require.Error(t, err)

		// test changing default key
		service = kms.NewService(kms.Config{
			MockClient:       true,
			KeyInfos:         ki,
			DefaultMasterKey: key2,
		})

		err = service.Initialize(ctx)
		require.NoError(t, err)

		encryptedPassphrase, encKeyID, err = service.GenerateEncryptedPassphrase(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, encryptedPassphrase)
		require.Equal(t, key2, encKeyID)

		decryptedPassphrase, err = service.DecryptPassphrase(ctx, encKeyID, encryptedPassphrase)
		require.NoError(t, err)
		require.NotEmpty(t, decryptedPassphrase)
	})
}
