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
	// BlockTypeEcPrivateKey is the value to define a block type of private key
	BlockTypeEcPrivateKey = "EC PRIVATE KEY"
	// BlockTypeCertificate is the value to define a block type of certificate
	BlockTypeCertificate = "CERTIFICATE"
	// BlockTypeIDOptions is the value to define a block type of id options
	// (e.g. `version`
	BlockTypeIDOptions = "ID OPTIONS"
)

// NewKeyBlock converts an ASN1/DER-encoded byte-slice of a private key into
// a `pem.Block` pointer
func NewKeyBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeEcPrivateKey, Bytes: b}
}

// NewCertBlock converts an ASN1/DER-encoded byte-slice of a tls certificate
// into a `pem.Block` pointer
func NewCertBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeCertificate, Bytes: b}
}

// KeyToDERBytes converts a private key to an ASN1/DER-encoded byte-slice
func KeyToDERBytes(key *ecdsa.PrivateKey) ([]byte, error) {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, errs.New("unable to marshal ECDSA private key", err)
	}

	return b, nil
}
