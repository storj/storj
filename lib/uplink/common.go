// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import "storj.io/storj/pkg/storj"

// Encryption specifies an individual bucket's encryption choices
type Encryption struct {
	Key           storj.Key
	EncPathPrefix storj.Path
	PathCipher    storj.Cipher
}

// APIKey is an interface for authenticating with the Satellite
type APIKey interface {
	Serialize() ([]byte, error)
}
