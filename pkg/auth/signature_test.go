// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/pkcrypto"
)

func TestGenerateSignature(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testidentity.IdentityVersionsTest(t, func(t *testing.T, version storj.IDVersion, _ *identity.FullIdentity) {
		ca, err := testidentity.NewTestCA(ctx, version.Number)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)

		for _, tt := range []struct {
			data     []byte
			verified bool
		}{
			{identity.ID.Bytes(), true},
			{[]byte("non verifiable data"), false},
		} {
			signature, err := GenerateSignature(identity.ID.Bytes(), identity)
			assert.NoError(t, err)

			verifyError := pkcrypto.HashAndVerifySignature(identity.Leaf.PublicKey, tt.data, signature)
			if tt.verified {
				assert.NoError(t, verifyError)
			} else {
				assert.Error(t, verifyError)
			}
		}
	})
}
