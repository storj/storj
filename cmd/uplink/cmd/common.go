// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

// loadEncryptionAccess loads the encryption key stored in the file pointed by
// filepath and creates an EncryptionAccess with it.
func loadEncryptionAccess(filepath string) (libuplink.EncryptionAccess, error) {
	key, err := uplink.LoadEncryptionKey(filepath)
	if err != nil {
		return libuplink.EncryptionAccess{}, err
	}

	return libuplink.EncryptionAccess{
		Key: *key,
	}, nil
}

// useOrLoadEncryptionAccess creates an encryption key from humanReadableKey
// when it isn't empty otherwise try to load the key from the file pointed by
// filepath and creates an EnryptionAccess with it.
func useOrLoadEncryptionAccess(humanReadableKey string, filepath string) (libuplink.EncryptionAccess, error) {
	if humanReadableKey != "" {
		key, err := storj.NewKey([]byte(humanReadableKey))
		if err != nil {
			return libuplink.EncryptionAccess{}, err
		}

		return libuplink.EncryptionAccess{
			Key: *key,
		}, nil
	}

	return loadEncryptionAccess(filepath)
}
