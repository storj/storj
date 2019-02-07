// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import (
	"crypto/rand"

	"github.com/spacemonkeygo/openssl"

	"storj.io/fork/crypto"
	"storj.io/fork/crypto/ecdsa"
	"storj.io/fork/crypto/elliptic"
	"storj.io/fork/crypto/rsa"
)

const (
	// StorjRSAKeyBits holds the number of bits to use for new RSA keys
	// by default.
	StorjRSAKeyBits = 2048
)

var (
	authECCurve = elliptic.P256()
)

// GeneratePrivateKey returns a new PrivateKey for signing messages
func GeneratePrivateKey() (crypto.PrivateKey, error) {
	return GeneratePrivateECDSAKey(authECCurve)
	// return GeneratePrivateRSAKey(StorjRSAKeyBits)
}

// GeneratePrivateECDSAKey returns a new private ECDSA key for signing messages
func GeneratePrivateECDSAKey(curve elliptic.Curve) (crypto.PrivateKey, error) {
	return ecdsa.GenerateKey(curve, rand.Reader)
}

// GeneratePrivateRSAKey returns a new private RSA key for signing messages
func GeneratePrivateRSAKey(bits int) (crypto.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

// HashAndVerifySignature checks that signature was made by the private key
// corresponding to the given public key, over a SHA-256 digest of the given
// data. It returns an error if verification fails, or nil otherwise.
func HashAndVerifySignature(key crypto.PublicKey, data, signature []byte) error {
	return key.VerifyPKCS1v15(crypto.SHA256, data, signature)
}

// PublicKeyFromPrivate returns the public key corresponding to a given private
// key.
func PublicKeyFromPrivate(privKey crypto.PrivateKey) crypto.PublicKey {
	return privKey.(openssl.PrivateKey)
}

// HashAndSign signs a SHA-256 digest of the given data and returns the new
// signature.
func HashAndSign(key crypto.PrivateKey, data []byte) ([]byte, error) {
	return key.SignPKCS1v15(crypto.SHA256, data)
}

// PublicKeyEqual returns true if two public keys are the same.
func PublicKeyEqual(a, b crypto.PublicKey) bool {
	return a.Equal(b)
}
