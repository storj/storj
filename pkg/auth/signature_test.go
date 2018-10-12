// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"testing"

	"github.com/gtank/cryptopasta"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/provider"
)

var (
	errorVerification = fmt.Errorf("Failed to verify signature")
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

func TestSignatureAuthVerifier(t *testing.T) {
	ctx := context.Background()
	ca, err := provider.NewCA(ctx, 12, 4)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	signature, err := GenerateSignature(identity)
	assert.NoError(t, err)

	peerIdentity := &provider.PeerIdentity{ID: identity.ID, Leaf: identity.Leaf}
	auth, err := NewSignatureAuth(signature, peerIdentity)
	assert.NoError(t, err)

	for _, tt := range []struct {
		signature []byte
		data      []byte
		publicKey []byte
		err       error
	}{
		{auth.Signature, auth.Data, auth.PublicKey, nil},
		{nil, auth.Data, auth.PublicKey, errorVerification},
		{auth.Signature, nil, auth.PublicKey, errorVerification},
		{auth.Signature, auth.Data, nil, errorVerification},

		{auth.Signature, []byte("malformed data"), auth.PublicKey, errorVerification},
	} {
		auth.Signature = tt.signature
		auth.Data = tt.data
		auth.PublicKey = tt.publicKey

		err := NewSignatureAuthVerifier()(auth)
		assert.Equal(t, tt.err, err)
	}
}
