// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/zeebo/errs"
)

const (
	BlockTypeEcPrivateKey = "EC PRIVATE KEY"
	BlockTypeCertificate  = "CERTIFICATE"
)

func NewKeyBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeEcPrivateKey, Bytes: b}
}

func NewCertBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeCertificate, Bytes: b}
}

func KeyToDERBytes(key *ecdsa.PrivateKey) ([]byte, error) {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, errs.New("unable to marshal ECDSA private key", err)
	}

	return b, nil
}

func keyToBlock(key *ecdsa.PrivateKey) (*pem.Block, error) {
	b, err := KeyToDERBytes(key)
	if err != nil {
		return nil, err
	}

	return NewKeyBlock(b), nil
}
