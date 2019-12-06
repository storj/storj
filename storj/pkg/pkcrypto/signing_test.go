// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSigningAndVerifyingECDSA(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"empty", ""},
		{"single byte", "C"},
		{"longnulls", string(make([]byte, 2000))},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			privKey, err := GeneratePrivateECDSAKey(authECCurve)
			assert.NoError(t, err)
			pubKey := PublicKeyFromPrivate(privKey)

			// test signing and verifying a hash of the data
			sig, err := HashAndSign(privKey, []byte(test.data))
			assert.NoError(t, err)
			err = HashAndVerifySignature(pubKey, []byte(test.data), sig)
			assert.NoError(t, err)

			// test signing and verifying the data directly
			sig, err = SignWithoutHashing(privKey, []byte(test.data))
			assert.NoError(t, err)
			err = VerifySignatureWithoutHashing(pubKey, []byte(test.data), sig)
			assert.NoError(t, err)
		})
	}
}

func TestSigningAndVerifyingRSA(t *testing.T) {
	privKey, err := GeneratePrivateRSAKey(StorjRSAKeyBits)
	assert.NoError(t, err)
	pubKey := PublicKeyFromPrivate(privKey)

	tests := []struct {
		name string
		data string
	}{
		{"empty", ""},
		{"single byte", "C"},
		{"longnulls", string(make([]byte, 2000))},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			// test signing and verifying a hash of the data
			sig, err := HashAndSign(privKey, []byte(test.data))
			assert.NoError(t, err)
			err = HashAndVerifySignature(pubKey, []byte(test.data), sig)
			assert.NoError(t, err)

			// don't test signing and verifying the data directly, as RSA can't
			// handle messages of arbitrary size
		})
	}
}
