// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"crypto/ecdsa"

	"github.com/gtank/cryptopasta"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/provider"
)

// SignedMessageProvider interface provides access to last signed message
type SignedMessageProvider interface {
	SignedMessage() (*pb.SignedMessage, error)
}

// GenerateSignature creates signature from identity id
func GenerateSignature(identity *provider.FullIdentity) ([]byte, error) {
	if identity == nil {
		return nil, nil
	}

	k, ok := identity.Key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, peertls.ErrUnsupportedKey.New("%T", identity.Key)
	}
	signature, err := cryptopasta.Sign(identity.ID.Bytes(), k)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

// NewSignedMessage creates instance of signed message
func NewSignedMessage(signature []byte, identity *provider.PeerIdentity) (*pb.SignedMessage, error) {
	k, ok := identity.Leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, peertls.ErrUnsupportedKey.New("%T", identity.Leaf.PublicKey)
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
			return errs.New("No message to verify")
		}
		if signedMessage.Signature == nil {
			return errs.New("Missing signature for verification")
		}
		if signedMessage.Data == nil {
			return errs.New("Missing data for verification")
		}
		if signedMessage.PublicKey == nil {
			return errs.New("Missing public key for verification")
		}

		k, err := cryptopasta.DecodePublicKey(signedMessage.GetPublicKey())
		if err != nil {
			return err
		}
		if ok := cryptopasta.Verify(signedMessage.GetData(), signedMessage.GetSignature(), k); !ok {
			return errs.New("Failed to verify message")
		}
		return nil
	}
}
