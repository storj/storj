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

	signature, err := GenerateSignature(identity.ID.Bytes(), identity)
	assert.NoError(t, err)

	signedMessage, err := NewSignedMessage(signature, identity)
	assert.NoError(t, err)

	for _, tt := range []struct {
		signature []byte
		data      []byte
		publicKey []byte
		errString string
	}{
		{signedMessage.Signature, signedMessage.Data, signedMessage.PublicKey, ""},
		{nil, signedMessage.Data, signedMessage.PublicKey, "auth error: missing signature for verification"},
		{signedMessage.Signature, nil, signedMessage.PublicKey, "auth error: missing data for verification"},
		{signedMessage.Signature, signedMessage.Data, nil, "auth error: missing public key for verification"},

		{signedMessage.Signature, []byte("malformed data"), signedMessage.PublicKey, "signature verification error: signature is not valid"},
	} {
		signedMessage.Signature = tt.signature
		signedMessage.Data = tt.data
		signedMessage.PublicKey = tt.publicKey

		err := NewSignedMessageVerifier()(signedMessage)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString)
		} else {
			assert.NoError(t, err)
		}
	}
}
