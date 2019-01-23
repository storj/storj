// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import (
	"crypto/ecdsa"
	"crypto/x509"

	"github.com/gtank/cryptopasta"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/storj"
)

var (
	//SatPrivKey wraps errors related to satellite private keys
	SatPrivKey = errs.Class("Processing satellite private key")
	//UpPrivKey wraps errors related to uplink private keys
	UpPrivKey = errs.Class("Processing uplink private key")
	//Renter wraps errors related to renter bandwidth allocations
	Renter = errs.Class("Renter agreement")
	//Payer wraps errors related to payer bandwidth allocations
	Payer = errs.Class("Payer agreement")
	//Missing indicates missing or empty information
	Missing = errs.Class("Required field is empty")
	//ECDSA indicates a key was not an ECDSA key
	ECDSA = errs.New("Key is not ecdsa key")
	//Sign indicates a failure during signing
	Sign = errs.Class("Failed to sign message")
	//Verify indicates a failure during signature validation
	Verify = errs.Class("Failed to validate message signature")
	//Marshal indicates a failure during serialization
	Marshal = errs.Class("Could not generate byte array from key")
	//Unmarshal indicates a failure during deserialization
	Unmarshal = errs.Class("Could not generate key from byte array")
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
)

//SignedMsg interface has a key, data, and signature
type SignedMsg interface {
	GetCerts() [][]byte
	GetData() []byte
	GetSignature() []byte
}

//VerifyMsg checks the crypto-related aspects of signed message
func VerifyMsg(sm SignedMsg, signer storj.NodeID) error {
	//no null fields
	certs, data, signature := sm.GetCerts(), sm.GetData(), sm.GetSignature()
	if certs == nil {
		return Missing.New("signer certificates")
	} else if data == nil {
		return Missing.New("message data")
	} else if signature == nil {
		return Missing.New("message signature")
	}
	if len(certs) < 2 {
		return Verify.New("Expected at least leaf and CA public keys")
	}
	//correct signature length
	err := peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)(certs, nil)
	if err != nil {
		return Verify.Wrap(err)
	}
	leafPubKey, err := parseECDSA(certs[0])
	if err != nil {
		return Unmarshal.Wrap(err)
	}
	caPubKey, err := parseECDSA(certs[1])
	if err != nil {
		return Unmarshal.Wrap(err)
	}
	signatureLength := leafPubKey.Curve.Params().P.BitLen() / 8
	if len(sm.GetSignature()) < signatureLength {
		return SigLen.New("%s", sm.GetSignature())
	}
	// verify signature
	if id, err := identity.NodeIDFromECDSAKey(caPubKey); err != nil || id != signer {
		return Signer.New("%+v vs %+v", id, signer)
	}
	if ok := cryptopasta.Verify(data, signature, leafPubKey); !ok {
		return Verify.New("%+v", ok)
	}
	return nil
}

func parseECDSA(rawCert []byte) (*ecdsa.PublicKey, error) {
	cert, err := x509.ParseCertificate(rawCert)
	if err != nil {
		return nil, Unmarshal.Wrap(err)
	}
	ecdsa, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, ECDSA
	}
	return ecdsa, nil
}
