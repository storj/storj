// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pkcrypto"
)

// GenerateSignature creates signature from identity id
func GenerateSignature(data []byte, identity *identity.FullIdentity) ([]byte, error) {
	return pkcrypto.HashAndSign(identity.Key, data)
}

// NewSignedMessage creates instance of signed message
func NewSignedMessage(signature []byte, identity *identity.FullIdentity) (*pb.SignedMessage, error) {
	encodedKey, err := pkcrypto.PublicKeyToPKIX(identity.Leaf.PublicKey)
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

		k, err := pkcrypto.PublicKeyFromPKIX(signedMessage.GetPublicKey())
		if err != nil {
			return Error.Wrap(err)
		}
		return pkcrypto.HashAndVerifySignature(k, signedMessage.GetData(), signedMessage.GetSignature())
	}
}
