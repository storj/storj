// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/kms"
)

func TestService(t *testing.T) {
	ctx := testcontext.New(t)

	service := kms.NewService(kms.Config{
		TestMasterKey: "test-master-key",
	})

	err := service.Initialize(ctx)
	require.NoError(t, err)

	encryptedPassphrase, err := service.GenerateEncryptedPassphrase(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, encryptedPassphrase)

	decryptedPassphrase, err := service.DecryptPassphrase(ctx, encryptedPassphrase)
	require.NoError(t, err)
	require.NotEmpty(t, decryptedPassphrase)

	// encrypting the decrypted passphrase should result in a different encrypted passphrase
	newEncryptedPassphrase, err := service.EncryptPassphrase(ctx, decryptedPassphrase)
	require.NoError(t, err)
	require.NotEqual(t, encryptedPassphrase, newEncryptedPassphrase)

	newDecryptedPassphrase, err := service.DecryptPassphrase(ctx, newEncryptedPassphrase)
	require.NoError(t, err)
	require.Equal(t, decryptedPassphrase, newDecryptedPassphrase)

	// malformed encrypted passphrase should return an error
	wrongEncryptedPassphrase := encryptedPassphrase
	wrongEncryptedPassphrase = append(wrongEncryptedPassphrase, []byte("random")...)
	_, err = service.DecryptPassphrase(ctx, wrongEncryptedPassphrase)
	require.Error(t, err)

	wrongEncryptedPassphrase = encryptedPassphrase
	wrongEncryptedPassphrase = append([]byte("random"), wrongEncryptedPassphrase...)
	_, err = service.DecryptPassphrase(ctx, wrongEncryptedPassphrase)
	require.Error(t, err)

	service = kms.NewService(kms.Config{
		TestMasterKey: "new-test-master-key",
	})

	err = service.Initialize(ctx)
	require.NoError(t, err)

	// service initialized with a different master key should not be able to decrypt the passphrase.
	_, err = service.DecryptPassphrase(ctx, encryptedPassphrase)
	require.Error(t, err)
}
