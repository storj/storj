// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pkcrypto"
)

// GenerateSignature creates signature from identity id
func GenerateSignature(ctx context.Context, data []byte, identity *identity.FullIdentity) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	return pkcrypto.HashAndSign(identity.Key, data)
}
