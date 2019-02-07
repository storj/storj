// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"io"

	"github.com/zeebo/errs"
)

// WriteKey writes the private key to the writer, PEM-encoded.
func WriteKey(w io.Writer, key crypto.PrivateKey) error {
	var (
		kb  []byte
		err error
	)

	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		kb, err = x509.MarshalECPrivateKey(k)
		if err != nil {
			return errs.Wrap(err)
		}
	default:
		return ErrUnsupportedKey.New("%T", k)
	}

	if err := pem.Encode(w, NewKeyBlock(kb)); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

// KeyBytes returns bytes of the private key to the writer, PEM-encoded.
func KeyBytes(key crypto.PrivateKey) ([]byte, error) {
	var data bytes.Buffer
	err := WriteKey(&data, key)
	return data.Bytes(), err
}

// NewKeyBlock converts an ASN1/DER-encoded byte-slice of a private key into
// a `pem.Block` pointer.
func NewKeyBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeEcPrivateKey, Bytes: b}
}

// NewCertBlock converts an ASN1/DER-encoded byte-slice of a tls certificate
// into a `pem.Block` pointer.
func NewCertBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeCertificate, Bytes: b}
}

// NewExtensionBlock converts an ASN1/DER-encoded byte-slice of a tls certificate
// extension into a `pem.Block` pointer.
func NewExtensionBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeExtension, Bytes: b}
}

// ParseCertificates parses an x509 certificate from each of the given byte
// slices, which should be encoded in DER. (Unwrap PEM encoding first if
// necessary.)
func ParseCertificates(rawCerts [][]byte) ([]*x509.Certificate, error) {
	certs := make([]*x509.Certificate, len(rawCerts))
	for i, c := range rawCerts {
		var err error
		certs[i], err = x509.ParseCertificate(c)
		if err != nil {
			return nil, ErrParseCerts.New("unable to parse certificate at index %d", i)
		}
	}
	return certs, nil
}
