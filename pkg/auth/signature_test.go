// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
	"crypto/ecdsa"
	"testing"

	"github.com/gtank/cryptopasta"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testidentity"
)

func TestGenerateSignature(t *testing.T) {
	ctx := context.Background()
	ca, err := testidentity.NewTestCA(ctx)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	k, ok := identity.Leaf.PublicKey.(*ecdsa.PublicKey)
	assert.Equal(t, true, ok)

	for _, tt := range []struct {
		data     []byte
		verified bool
	}{
		{identity.ID.Bytes(), true},
		{[]byte("non verifiable data"), false},
	} {
		signature, err := GenerateSignature(identity.ID.Bytes(), identity)
		assert.NoError(t, err)

		verified := cryptopasta.Verify(tt.data, signature, k)
		assert.Equal(t, tt.verified, verified)
	}
}
