// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import (
	"encoding/asn1"
	"encoding/pem"
	"io"
	"math/big"

	"github.com/zeebo/errs"

	"storj.io/fork/crypto"
	"storj.io/fork/crypto/x509"
)

// WritePublicKeyPEM writes the public key, in a PEM-enveloped
// PKIX form.
func WritePublicKeyPEM(w io.Writer, key crypto.PublicKey) error {
	keyBytes, err := PublicKeyToPEM(key)
	if err != nil {
		return errs.Wrap(err)
	}
	_, err = w.Write(keyBytes)
	return err
}

// PublicKeyToPEM encodes a public key to a PEM-enveloped PKIX form.
func PublicKeyToPEM(key crypto.PublicKey) ([]byte, error) {
	return key.MarshalPKIXPublicKeyPEM()
}

// PublicKeyToPKIX serializes a public key to a PKIX-encoded form.
func PublicKeyToPKIX(key crypto.PublicKey) ([]byte, error) {
	return key.MarshalPKIXPublicKeyDER()
}

// PublicKeyFromPKIX parses a public key from its PKIX encoding.
func PublicKeyFromPKIX(pkixData []byte) (crypto.PublicKey, error) {
	return x509.ParsePKIXPublicKey(pkixData)
}

// PublicKeyFromPEM parses a public key from its PEM-enveloped PKIX
// encoding.
func PublicKeyFromPEM(pemData []byte) (crypto.PublicKey, error) {
	return x509.ParsePEMPublicKey(pemData)
}

// WritePrivateKeyPEM writes the private key to the writer, in a PEM-enveloped
// PKCS#8 form.
func WritePrivateKeyPEM(w io.Writer, key crypto.PrivateKey) error {
	keyBytes, err := PrivateKeyToPEM(key)
	if err != nil {
		return errs.Wrap(err)
	}
	_, err = w.Write(keyBytes)
	return errs.Wrap(err)
}

// PrivateKeyToPEM serializes a private key to a PEM-enveloped PKCS#8 form.
func PrivateKeyToPEM(key crypto.PrivateKey) ([]byte, error) {
	return key.MarshalPKCS1PrivateKeyPEM()
}

// PrivateKeyToPKCS8 serializes a private key to a PKCS#8-encoded form.
func PrivateKeyToPKCS8(key crypto.PrivateKey) ([]byte, error) {
	return key.MarshalPKCS1PrivateKeyDER()
}

// PrivateKeyFromPKCS8 parses a private key from its PKCS#8 encoding.
func PrivateKeyFromPKCS8(keyBytes []byte) (crypto.PrivateKey, error) {
	return x509.ParsePKCS8PrivateKey(keyBytes)
}

// PrivateKeyFromPEM parses a private key from its PEM-enveloped PKCS#8
// encoding.
func PrivateKeyFromPEM(pemBytes []byte) (crypto.PrivateKey, error) {
	return x509.ParsePEMPrivateKey(pemBytes)
}

// WriteCertPEM writes the certificate(s) to the writer, in a PEM-enveloped DER
// encoding.
func WriteCertPEM(w io.Writer, certs ...*x509.Certificate) error {
	if len(certs) == 0 {
		return errs.New("no certs to encode")
	}
	encodeErrs := new(errs.Group)
	for _, cert := range certs {
		certBytes, err := CertToPEM(cert)
		if err != nil {
			encodeErrs.Add(err)
			continue
		}
		_, err = w.Write(certBytes)
		if err != nil {
			encodeErrs.Add(err)
		}
	}
	return encodeErrs.Err()
}

// CertToPEM returns the bytes of the certificate, in a PEM-enveloped DER
// encoding.
func CertToPEM(cert *x509.Certificate) ([]byte, error) {
	return cert.MarshalPEM()
}

// CertToDER returns the bytes of the certificate, in a DER encoding.
func CertToDER(cert *x509.Certificate) ([]byte, error) {
	return cert.MarshalDER()
}

// CertFromDER parses an X.509 certificate from its DER encoding.
func CertFromDER(certDER []byte) (*x509.Certificate, error) {
	return x509.ParseCertificate(certDER)
}

// CertFromPEM parses an X.509 certificate from its PEM-enveloped DER encoding.
func CertFromPEM(certPEM []byte) (*x509.Certificate, error) {
	return x509.ParsePEMCertificate(certPEM)
}

// CertsFromDER parses an x509 certificate from each of the given byte
// slices, which should be encoded in DER.
func CertsFromDER(rawCerts [][]byte) ([]*x509.Certificate, error) {
	certs := make([]*x509.Certificate, len(rawCerts))
	for i, c := range rawCerts {
		var err error
		certs[i], err = CertFromDER(c)
		if err != nil {
			return nil, ErrParse.New("unable to parse certificate at index %d", i)
		}
	}
	return certs, nil
}

// CertsFromPEM parses a PEM chain from a single byte string (the PEM-enveloped
// certificates should be concatenated). The PEM blocks may include PKIX
// extensions.
func CertsFromPEM(pemBytes []byte) ([]*x509.Certificate, error) {
	var (
		encChain  encodedChain
		blockErrs errs.Group
	)
	for {
		var pemBlock *pem.Block
		pemBlock, pemBytes = pem.Decode(pemBytes)
		if pemBlock == nil {
			break
		}
		switch pemBlock.Type {
		case BlockLabelCertificate:
			encChain.AddCert(pemBlock.Bytes)
		}
	}
	if err := blockErrs.Err(); err != nil {
		return nil, err
	}

	return encChain.Parse()
}

type encodedChain struct {
	chain [][]byte
}

func (e *encodedChain) AddCert(b []byte) {
	e.chain = append(e.chain, b)
}

func (e *encodedChain) Parse() ([]*x509.Certificate, error) {
	chain, err := CertsFromDER(e.chain)
	if err != nil {
		return nil, err
	}

	return chain, nil
}

type ecdsaSignature struct {
	R, S *big.Int
}

func marshalECDSASignature(r, s *big.Int) ([]byte, error) {
	return asn1.Marshal(ecdsaSignature{R: r, S: s})
}

func unmarshalECDSASignature(signatureBytes []byte) (r, s *big.Int, err error) {
	var signature ecdsaSignature
	if _, err = asn1.Unmarshal(signatureBytes, &signature); err != nil {
		return nil, nil, err
	}
	return signature.R, signature.S, nil
}
