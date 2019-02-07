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
		t.Run(tt.name, func(t *testing.T) {
			privKey, err := GeneratePrivateECDSAKey(authECCurve)
			assert.NoError(t, err)

			// test signing and verifying a hash of the data
			sig, err := HashAndSign(privKey, []byte(tt.data))
			assert.NoError(t, err)
			err = HashAndVerifySignature(PublicKeyFromPrivate(privKey), []byte(tt.data), sig)
			assert.NoError(t, err)
		})
	}
}

func TestSigningAndVerifyingRSA(t *testing.T) {
	privKey, err := GeneratePrivateRSAKey(StorjRSAKeyBits)
	assert.NoError(t, err)

	tests := []struct {
		name string
		data string
	}{
		{"empty", ""},
		{"single byte", "C"},
		{"longnulls", string(make([]byte, 2000))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test signing and verifying a hash of the data
			sig, err := HashAndSign(privKey, []byte(tt.data))
			assert.NoError(t, err)
			err = HashAndVerifySignature(PublicKeyFromPrivate(privKey), []byte(tt.data), sig)
			assert.NoError(t, err)
		})
	}
}
