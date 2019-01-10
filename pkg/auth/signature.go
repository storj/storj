// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"crypto/ecdsa"

	"github.com/gtank/cryptopasta"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/provider"
)

// GenerateSignature creates signature from identity id
func GenerateSignature(data []byte, identity *provider.FullIdentity) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	k, ok := identity.Key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, peertls.ErrUnsupportedKey.New("%T", identity.Key)
	}
	signature, err := cryptopasta.Sign(data, k)
	if err != nil {
		return nil, err
	}
	return signature, nil
}
