// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"crypto/ecdsa"

	"github.com/gtank/cryptopasta"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pkcrypto"
)

// GenerateSignature creates signature from identity id
func GenerateSignature(data []byte, identity *identity.FullIdentity) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	k, ok := identity.Key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, pkcrypto.ErrUnsupportedKey.New("%T", identity.Key)
	}
	signature, err := cryptopasta.Sign(data, k)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

// NewSignedMessage creates instance of signed message
func NewSignedMessage(signature []byte, identity *identity.FullIdentity) (*pb.SignedMessage, error) {
	k, ok := identity.Leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, pkcrypto.ErrUnsupportedKey.New("%T", identity.Leaf.PublicKey)
	}

	encodedKey, err := cryptopasta.EncodePublicKey(k)
	if err != nil {
		return nil, err
	}
	return &pb.SignedMessage{
		Data:      identity.ID.Bytes(),
		Signature: signature,
		PublicKey: encodedKey,
	}, nil
}

// SignedMessageVerifier checks if provided signed message can be verified
type SignedMessageVerifier func(signature *pb.SignedMessage) error

// NewSignedMessageVerifier creates default implementation of SignedMessageVerifier
func NewSignedMessageVerifier() SignedMessageVerifier {
	return func(signedMessage *pb.SignedMessage) error {
		if signedMessage == nil {
			return Error.New("no message to verify")
		}
		if signedMessage.Signature == nil {
			return Error.New("missing signature for verification")
		}
		if signedMessage.Data == nil {
			return Error.New("missing data for verification")
		}
		if signedMessage.PublicKey == nil {
			return Error.New("missing public key for verification")
		}

		k, err := cryptopasta.DecodePublicKey(signedMessage.GetPublicKey())
		if err != nil {
			return Error.Wrap(err)
		}
		if ok := cryptopasta.Verify(signedMessage.GetData(), signedMessage.GetSignature(), k); !ok {
			return Error.New("failed to verify message")
		}
		return nil
	}
}
