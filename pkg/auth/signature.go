// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pkcrypto"
)

// GenerateSignature creates signature from identity id
func GenerateSignature(data []byte, identity *identity.FullIdentity) ([]byte, error) {
	return pkcrypto.HashAndSign(identity.Key, data)
}
