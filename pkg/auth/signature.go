// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"crypto/ecdsa"
	"encoding/base64"

	"github.com/gtank/cryptopasta"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/provider"
)

// SignatureAuthProvider interface provides access to last signature auth data
type SignatureAuthProvider interface {
	Auth() (*pb.SignatureAuth, error)
}

// GenerateSignature creates signature from identity id
func GenerateSignature(identity *provider.FullIdentity) (string, error) {
	if identity == nil {
		return "", nil
	}

	k, ok := identity.Key.(*ecdsa.PrivateKey)
	if !ok {
		return "", peertls.ErrUnsupportedKey.New("%T", identity.Key)
	}
	signature, err := cryptopasta.Sign(identity.ID.Bytes(), k)
	if err != nil {
		return "", nil
	}
	base64 := base64.StdEncoding
	encodedSignature := base64.EncodeToString(signature)
	return encodedSignature, nil
}

// NewSignatureAuth creates instance of signature auth data
func NewSignatureAuth(signature []byte, identity *provider.PeerIdentity) (*pb.SignatureAuth, error) {
	k, ok := identity.Leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, peertls.ErrUnsupportedKey.New("%T", identity.Leaf.PublicKey)
	}

	encodedKey, err := cryptopasta.EncodePublicKey(k)
	if err != nil {
		return nil, err
	}
	return &pb.SignatureAuth{
		Data:      identity.ID.Bytes(),
		Signature: signature,
		PublicKey: encodedKey,
	}, nil
}
