// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/pkcrypto"
)

func TestGenerateSignature(t *testing.T) {
	ctx := context.Background()
	ca, err := testidentity.NewTestCA(ctx)
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
}

func TestSignedMessageVerifier(t *testing.T) {
	ctx := context.Background()
	ca, err := testidentity.NewTestCA(ctx)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	_, err = GenerateSignature(identity.ID.Bytes(), identity)
	assert.NoError(t, err)
}
