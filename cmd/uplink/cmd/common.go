// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

func newEncryptionAccess(keyData string) (libuplink.EncryptionAccess, error) {
	key, err := storj.NewKey([]byte(keyData))
	if err != nil {
		return libuplink.EncryptionAccess{}, err
	}
	return libuplink.EncryptionAccess{Key: *key}, nil
}
