// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"

	"github.com/zeebo/errs"
)

const (
	BlockTypeEcPrivateKey = "EC PRIVATE KEY"
	BlockTypeCertificate  = "CERTIFICATE"
)

func newKeyBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeEcPrivateKey, Bytes: b}
}

func newCertBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeCertificate, Bytes: b}
}

func keyToDERBytes(key *ecdsa.PrivateKey) ([]byte, error) {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, errs.New("unable to marshal ECDSA private key", err)
	}

	return b, nil
}

func keyToBlock(key *ecdsa.PrivateKey) (*pem.Block, error) {
	b, err := keyToDERBytes(key)
	if err != nil {
		return nil, err
	}

	return newKeyBlock(b), nil
}

func certFromPEMs(certPEMBytes, keyPEMBytes []byte) (*tls.Certificate, error) {
	certDERs := [][]byte{}

	for {
		var certDERBlock *pem.Block

		certDERBlock, certPEMBytes = pem.Decode(certPEMBytes)
		if certDERBlock == nil {
			break
		}

		certDERs = append(certDERs, certDERBlock.Bytes)
	}

	keyPEMBlock, _ := pem.Decode(keyPEMBytes)
	if keyPEMBlock == nil {
		return nil, errs.New("unable to decode key PEM data")
	}

	return certFromDERs(certDERs, keyPEMBlock.Bytes)
}

func certFromDERs(certDERBytes [][]byte, keyDERBytes []byte) (*tls.Certificate, error) {
	var (
		err  error
		cert = new(tls.Certificate)
	)

	cert.Certificate = certDERBytes
	cert.PrivateKey, err = x509.ParseECPrivateKey(keyDERBytes)
	if err != nil {
		return nil, errs.New("unable to parse EC private key", err)
	}

	return cert, nil
}
