// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/asn1"
	"math/big"
)

// ECDSASignature holds the `r` and `s` values in an ecdsa signature
// (see https://golang.org/pkg/crypto/ecdsa)
type ECDSASignature struct {
	R, S *big.Int
}

var authECCurve = elliptic.P256()

// GeneratePrivateKey returns a new PrivateKey for signing messages
func GeneratePrivateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(authECCurve, rand.Reader)
}

// VerifySignature checks the signature against the passed data and public key
func VerifySignature(signedData, data []byte, pubKey crypto.PublicKey) error {
	key, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return ErrUnsupportedKey.New("%T", key)
	}

	signature := new(ECDSASignature)
	if _, err := asn1.Unmarshal(signedData, signature); err != nil {
		return ErrVerifySignature.New("unable to unmarshal ecdsa signature: %v", err)
	}
	digest := SHA256Hash(data)
	if !ecdsa.Verify(key, digest, signature.R, signature.S) {
		return ErrVerifySignature.New("signature is not valid")
	}
	return nil
}

// SignBytes signs the given data with the private key and returns the new
// signature. Normally, data here is a digest of some longer string of bytes.
func SignBytes(key crypto.PrivateKey, data []byte) ([]byte, error) {
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, ErrUnsupportedKey.New("%T", key)
	}

	r, s, err := ecdsa.Sign(rand.Reader, ecKey, data)
	if err != nil {
		return nil, ErrSign.Wrap(err)
	}

	return asn1.Marshal(ECDSASignature{R: r, S: s})
}

// SignHashOf signs a SHA-256 digest of the given data and returns the new
// signature.
func SignHashOf(key crypto.PrivateKey, data []byte) ([]byte, error) {
	hash := SHA256Hash(data)
	signature, err := SignBytes(key, hash)
	if err != nil {
		return nil, ErrSign.Wrap(err)
	}
	return signature, nil
}
