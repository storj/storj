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
	errorVerification = fmt.Errorf("Failed to verify message")
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

func TestSignedMessageVerifier(t *testing.T) {
	ctx := context.Background()
	ca, err := provider.NewCA(ctx, 12, 4)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	signature, err := GenerateSignature(identity)
	assert.NoError(t, err)

	peerIdentity := &provider.PeerIdentity{ID: identity.ID, Leaf: identity.Leaf}
	signedMessage, err := NewSignedMessage(signature, peerIdentity)
	assert.NoError(t, err)

	for _, tt := range []struct {
		signature []byte
		data      []byte
		publicKey []byte
		err       error
	}{
		{signedMessage.Signature, signedMessage.Data, signedMessage.PublicKey, nil},
		{nil, signedMessage.Data, signedMessage.PublicKey, errorVerification},
		{signedMessage.Signature, nil, signedMessage.PublicKey, errorVerification},
		{signedMessage.Signature, signedMessage.Data, nil, errorVerification},

		{signedMessage.Signature, []byte("malformed data"), signedMessage.PublicKey, errorVerification},
	} {
		signedMessage.Signature = tt.signature
		signedMessage.Data = tt.data
		signedMessage.PublicKey = tt.publicKey

		err := NewSignedMessageVerifier()(signedMessage)
		assert.Equal(t, tt.err, err)
	}
}
