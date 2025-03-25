// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms_test

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/kms"
)

func TestService(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	id1 := 1
	id2 := 2

	key1, err := storj.NewKey([]byte("testkey1"))
	require.NoError(t, err)
	key2, err := storj.NewKey([]byte("testkey2"))
	require.NoError(t, err)

	t.Run("invalid provider", func(t *testing.T) {
		service := kms.NewService(kms.Config{
			Provider: "",
			KeyInfos: kms.KeyInfos{},
		})
		err := service.Initialize(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid encryption key provider")
	})

	providers := []string{"gsm", "local"}
	for _, p := range providers {
		var defaultCfg, checksumMismatchCfg, emptyKeyDataCfg string
		switch p {
		case "gsm":
			defaultCfg = fmt.Sprintf("%d:secretversion1,12345;%d:secretversion2,54321", id1, id2)
			checksumMismatchCfg = fmt.Sprintf("%d:%s,12345", id1, kms.MockChecksumMismatch)
			emptyKeyDataCfg = fmt.Sprintf("%d:%s,12345", id1, kms.MockKeyNotFound)
		case "local":
			crc32c := crc32.MakeTable(crc32.Castagnoli)
			checksum1 := crc32.Checksum(key1.Raw()[:], crc32c)
			checksum2 := crc32.Checksum(key2.Raw()[:], crc32c)

			key1FilePath := ctx.File("key1")
			key2FilePath := ctx.File("key2")
			emptyKeyFile := ctx.File("emptyKeyFile")

			require.NoError(t, os.WriteFile(key1FilePath, key1.Raw()[:], 0644))
			require.NoError(t, os.WriteFile(key2FilePath, key2.Raw()[:], 0644))
			_, err := os.Create(emptyKeyFile)
			require.NoError(t, err)

			defaultCfg = fmt.Sprintf("%d:%s,%d;", id1, key1FilePath, checksum1)
			defaultCfg += fmt.Sprintf("%d:%s,%d", id2, key2FilePath, checksum2)
			checksumMismatchCfg = fmt.Sprintf("%d:%s,11111", id1, key1FilePath)
			emptyKeyDataCfg = fmt.Sprintf("%d:%s,11111", id1, emptyKeyFile)
		default:
			t.Error("invalid secret provider")
		}

		var defaultKI, checksumMismatchKI, emptyKeyDataKI kms.KeyInfos
		require.NoError(t, defaultKI.Set(defaultCfg))
		require.NoError(t, checksumMismatchKI.Set(checksumMismatchCfg))
		require.NoError(t, emptyKeyDataKI.Set(emptyKeyDataCfg))

		t.Run(p+"_master key not set", func(t *testing.T) {
			service := kms.NewService(kms.Config{
				Provider:         p,
				MockClient:       true,
				KeyInfos:         defaultKI,
				DefaultMasterKey: 3,
			})
			err := service.Initialize(ctx)
			require.Error(t, err)
			require.Contains(t, err.Error(), "master key not set")
		})

		t.Run(p+"_encrypt/decrypt passphrases", func(t *testing.T) {
			service := kms.NewService(kms.Config{
				Provider:         p,
				MockClient:       true,
				KeyInfos:         defaultKI,
				DefaultMasterKey: id1,
			})

			err := service.Initialize(ctx)
			require.NoError(t, err)

			encryptedPassphrase, encKeyID, err := service.GenerateEncryptedPassphrase(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, encryptedPassphrase)
			require.Equal(t, id1, encKeyID)

			decryptedPassphrase, err := service.DecryptPassphrase(ctx, encKeyID, encryptedPassphrase)
			require.NoError(t, err)
			require.NotEmpty(t, decryptedPassphrase)

			// encrypting the decrypted passphrase should result in a different encrypted passphrase
			newEncryptedPassphrase, encKeyID, err := service.EncryptPassphrase(ctx, decryptedPassphrase)
			require.NoError(t, err)
			require.NotEqual(t, encryptedPassphrase, newEncryptedPassphrase)
			require.Equal(t, id1, encKeyID)

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
			_, err = service.DecryptPassphrase(ctx, id2, encryptedPassphrase)
			require.Error(t, err)

			// test changing default key
			service = kms.NewService(kms.Config{
				Provider:         p,
				MockClient:       true,
				KeyInfos:         defaultKI,
				DefaultMasterKey: id2,
			})

			err = service.Initialize(ctx)
			require.NoError(t, err)

			encryptedPassphrase, encKeyID, err = service.GenerateEncryptedPassphrase(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, encryptedPassphrase)
			require.Equal(t, id2, encKeyID)

			decryptedPassphrase, err = service.DecryptPassphrase(ctx, encKeyID, encryptedPassphrase)
			require.NoError(t, err)
			require.NotEmpty(t, decryptedPassphrase)
		})

		t.Run(p+"_checksum mismatch", func(t *testing.T) {
			service := kms.NewService(kms.Config{
				Provider:         p,
				MockClient:       true,
				KeyInfos:         checksumMismatchKI,
				DefaultMasterKey: id1,
			})
			err := service.Initialize(ctx)
			require.Error(t, err)
			require.Contains(t, err.Error(), "checksum mismatch")
		})

		t.Run(p+"_key not found", func(t *testing.T) {
			service := kms.NewService(kms.Config{
				Provider:         p,
				MockClient:       true,
				KeyInfos:         emptyKeyDataKI,
				DefaultMasterKey: id1,
			})
			err := service.Initialize(ctx)
			require.Error(t, err)
			if p == "gsm" {
				require.Contains(t, err.Error(), "no key found in secret manager")
			} else if p == "local" {
				require.Contains(t, err.Error(), "empty key data")
			}
		})

		if p == "local" {
			t.Run(p+"_error reading file", func(t *testing.T) {
				nonexistentFile := filepath.Join(ctx.Dir("testdir"), "nonexistent")
				nonexistentFileCfg := fmt.Sprintf("%d:%s,11111", id1, nonexistentFile)
				var nonexistentFileKI kms.KeyInfos
				require.NoError(t, nonexistentFileKI.Set(nonexistentFileCfg))
				service := kms.NewService(kms.Config{
					Provider:         p,
					MockClient:       true,
					KeyInfos:         nonexistentFileKI,
					DefaultMasterKey: id1,
				})
				err := service.Initialize(ctx)
				require.Error(t, err)
				require.Contains(t, err.Error(), "error reading local key file")
			})
		}
	}
}
