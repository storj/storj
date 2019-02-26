// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

var (
	//ErrSign indicates a failure during signing
	ErrSign = errs.Class("Failed to sign message")
	//ErrVerify indicates a failure during signature validation
	ErrVerify = errs.Class("Failed to validate message signature")
	//ErrSigLen indicates an invalid signature length
	ErrSigLen = errs.Class("Invalid signature length")
	//ErrSerial indicates an invalid serial number length
	ErrSerial = errs.Class("Invalid SerialNumber")
	//ErrExpired indicates the agreement is expired
	ErrExpired = errs.Class("Agreement is expired")
	//ErrSigner indicates a public key / node id mismatch
	ErrSigner = errs.Class("Message public key did not match expected signer")
	//ErrBadID indicates a public key / node id mismatch
	ErrBadID = errs.Class("Node ID did not match expected id")

	//ErrMarshal indicates a failure during serialization
	ErrMarshal = errs.Class("Could not marshal item to bytes")
	//ErrUnmarshal indicates a failure during deserialization
	ErrUnmarshal = errs.Class("Could not unmarshal bytes to item")
	//ErrMissing indicates missing or empty information
	ErrMissing = errs.Class("Required field is empty")
)

//SignableMessage is a protocol buffer with a certs and a signature
//Note that we assume proto.Message is a pointer receiver
type SignableMessage interface {
	proto.Message
	GetCerts() [][]byte
	GetSignature() []byte
	SetCerts([][]byte)
	SetSignature([]byte)
	GetSignedMessage() []byte
	SetSignedMessage([]byte)
}

//SignMessage adds the crypto-related aspects of signed message
func SignMessage(msg SignableMessage, ID identity.FullIdentity) error {
	if msg == nil {
		return ErrMissing.New("message")
	}
	msg.SetSignature(nil)
	msg.SetCerts(nil)
	msg.SetSignedMessage(nil)
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return ErrMarshal.Wrap(err)
	}
	signature, err := pkcrypto.HashAndSign(ID.Key, msgBytes)
	if err != nil {
		return ErrSign.Wrap(err)
	}
	msg.SetSignature(signature)
	msg.SetCerts(ID.ChainRaw())
	msg.SetSignedMessage(msgBytes)
	return nil
}

//VerifyMsg checks the crypto-related aspects of signed message
func VerifyMsg(msg SignableMessage, signer storj.NodeID) error {
	//setup
	if msg == nil {
		return ErrMissing.New("message")
	} else if msg.GetSignature() == nil {
		return ErrMissing.New("message signature")
	} else if msg.GetCerts() == nil {
		return ErrMissing.New("message certificates")
	} else if msg.GetSignedMessage() == nil {
		return ErrMissing.New("message certificates")
	}
	signature := msg.GetSignature()
	certs := msg.GetCerts()
	signedMessage := msg.GetSignedMessage()

	//check certs
	if len(certs) < 2 {
		return ErrVerify.New("Expected at least leaf and CA public keys")
	}
	err := peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)(certs, nil)
	if err != nil {
		return ErrVerify.Wrap(err)
	}
	leaf, err := pkcrypto.CertFromDER(certs[0])
	if err != nil {
		return err
	}
	ca, err := pkcrypto.CertFromDER(certs[1])
	if err != nil {
		return err
	}
	// verify signature
	if id, err := identity.NodeIDFromKey(ca.PublicKey); err != nil || id != signer {
		return ErrSigner.New("%+v vs %+v", id, signer)
	}
	if err := pkcrypto.HashAndVerifySignature(leaf.PublicKey, signedMessage, signature); err != nil {
		return ErrVerify.New("%+v", err)
	}
	//cleanup
	msg.SetSignature(nil)
	msg.SetCerts(nil)
	msg.SetSignedMessage(nil)
	defer msg.SetSignature(signature)
	defer msg.SetCerts(certs)
	defer msg.SetSignedMessage(signedMessage)
	//replace the existing message with the known valid one
	//todo:  don't have an existing message!
	if err = proto.Unmarshal(signedMessage, msg); err != nil {
		return ErrMarshal.Wrap(err)
	}
	return nil
}
