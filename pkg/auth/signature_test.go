// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
	"crypto/ecdsa"
	"testing"

	"github.com/gtank/cryptopasta"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/provider"
)

func TestGenerateSignature(t *testing.T) {
	ctx := context.Background()
	ca, err := provider.NewCA(ctx, 12, 4)
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
		signature, err := GenerateSignature(identity)
		assert.NoError(t, err)

		verified := cryptopasta.Verify(tt.data, signature, k)
		assert.Equal(t, tt.verified, verified)
	}
}
