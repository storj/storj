// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"crypto/ecdsa"
	"crypto/x509"

	"github.com/gogo/protobuf/proto"
	"github.com/gtank/cryptopasta"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/storj"
)

var (
	//ECDSA indicates a key was not an ECDSA key
	ECDSA = errs.New("Key is not ecdsa key")
	//Sign indicates a failure during signing
	Sign = errs.Class("Failed to sign message")
	//Verify indicates a failure during signature validation
	Verify = errs.Class("Failed to validate message signature")
	//SigLen indicates an invalid signature length
	SigLen = errs.Class("Invalid signature length")
	//Serial indicates an invalid serial number length
	Serial = errs.Class("Invalid SerialNumber")
	//Expired indicates the agreement is expired
	Expired = errs.Class("Agreement is expired")
	//Signer indicates a public key / node id mismatch
	Signer = errs.Class("Message public key did not match expected signer")
	//BadID indicates a public key / node id mismatch
	BadID = errs.Class("Node ID did not match expected id")

	//Marshal indicates a failure during serialization
	Marshal = errs.Class("Could not marshal item to bytes")
	//Unmarshal indicates a failure during deserialization
	Unmarshal = errs.Class("Could not unmarshal bytes to item")
	//Missing indicates missing or empty information
	Missing = errs.Class("Required field is empty")
)

//SignableMsg is a protocol buffer with a certs and a signature
//Note that we assume proto.Message is a pointer receiver
type SignableMsg interface {
	proto.Message
	GetCerts() [][]byte
	GetSignature() []byte
	SetCerts([][]byte) bool
	SetSignature([]byte) bool
}

//SignMsg adds the crypto-related aspects of signed message
func SignMsg(msg SignableMsg, ID identity.FullIdentity) error {
	if msg == nil {
		return Missing.New("message")
	}
	_ = msg.SetSignature(nil)
	_ = msg.SetCerts(nil)
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return Marshal.Wrap(err)
	}
	privECDSA, ok := ID.Key.(*ecdsa.PrivateKey)
	if !ok {
		return ECDSA
	}
	signature, err := cryptopasta.Sign(msgBytes, privECDSA)
	if err != nil {
		return Sign.Wrap(err)
	}
	_ = msg.SetSignature(signature)
	_ = msg.SetCerts(ID.ChainRaw())
	return nil
}

//VerifyMsg checks the crypto-related aspects of signed message
func VerifyMsg(msg SignableMsg, signer storj.NodeID) error {
	//setup
	if msg == nil {
		return Missing.New("message")
	} else if msg.GetSignature() == nil {
		return Missing.New("message signature")
	} else if msg.GetCerts() == nil {
		return Missing.New("message certificates")
	}
	signature := msg.GetSignature()
	certs := msg.GetCerts()
	_ = msg.SetSignature(nil)
	_ = msg.SetCerts(nil)
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return Marshal.Wrap(err)
	}
	//check certs
	if len(certs) < 2 {
		return Verify.New("Expected at least leaf and CA public keys")
	}
	err = peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)(certs, nil)
	if err != nil {
		return Verify.Wrap(err)
	}
	leafPubKey, err := parseECDSA(certs[0])
	if err != nil {
		return err
	}
	caPubKey, err := parseECDSA(certs[1])
	if err != nil {
		return err
	}
	// verify signature
	signatureLength := leafPubKey.Curve.Params().P.BitLen() / 8
	if len(signature) < signatureLength {
		return SigLen.New("%d vs %d", len(signature), signatureLength)
	}
	if id, err := identity.NodeIDFromECDSAKey(caPubKey); err != nil || id != signer {
		return Signer.New("%+v vs %+v", id, signer)
	}
	if ok := cryptopasta.Verify(msgBytes, signature, leafPubKey); !ok {
		return Verify.New("%+v", ok)
	}
	//cleanup
	_ = msg.SetSignature(signature)
	_ = msg.SetCerts(certs)
	return nil
}

func parseECDSA(rawCert []byte) (*ecdsa.PublicKey, error) {
	cert, err := x509.ParseCertificate(rawCert)
	if err != nil {
		return nil, Verify.Wrap(err)
	}
	ecdsa, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, ECDSA
	}
	return ecdsa, nil
}
