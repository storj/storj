// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

// Many cryptography standards use ASN.1 to define their data structures,
// and Distinguished Encoding Rules (DER) to serialize those structures.
// Because DER produces binary output, it can be challenging to transmit
// the resulting files through systems, like electronic mail, that only
// support ASCII. The PEM format solves this problem by encoding the
// binary data using base64.
// (see https://en.wikipedia.org/wiki/Privacy-enhanced_Electronic_Mail)

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"math/big"

	"github.com/zeebo/errs"
)

// ECDSASignature holds the `r` and `s` values in an ecdsa signature
// (see https://golang.org/pkg/crypto/ecdsa)
type ECDSASignature struct {
	R, S *big.Int
}

var authECCurve = elliptic.P256()

func parseCertificateChains(rawCerts [][]byte) ([]*x509.Certificate, error) {
	parsedCerts, err := parseCerts(rawCerts)
	if err != nil {
		return nil, err
	}

	return parsedCerts, nil
}

func parseCerts(rawCerts [][]byte) ([]*x509.Certificate, error) {
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

func verifyChainSignatures(certs []*x509.Certificate) error {
	for i, cert := range certs {
		j := len(certs)
		if i+1 < j {
			err := verifyCertSignature(certs[i+1], cert)
			if err != nil {
				return ErrVerifyCertificateChain.Wrap(err)
			}

			continue
		}

		err := verifyCertSignature(cert, cert)
		if err != nil {
			return ErrVerifyCertificateChain.Wrap(err)
		}

	}

	return nil
}

func verifyCertSignature(parentCert, childCert *x509.Certificate) error {
	return verifySignature(childCert.Signature, childCert.RawTBSCertificate, parentCert.PublicKey)
}

func verifySignature(signedData []byte, data []byte, pubKey crypto.PublicKey) error {
	key, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return ErrUnsupportedKey.New("%T", key)
	}

	signature := new(ECDSASignature)

	if _, err := asn1.Unmarshal(signedData, signature); err != nil {
		return ErrVerifySignature.New("unable to unmarshal ecdsa signature: %v", err)
	}

	h := crypto.SHA256.New()
	_, err := h.Write(data)
	if err != nil {
		return ErrVerifySignature.Wrap(err)
	}
	digest := h.Sum(nil)

	if !ecdsa.Verify(key, digest, signature.R, signature.S) {
		return ErrVerifySignature.New("signature is not valid")
	}
	return nil
}

func newSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errs.New("failed to generateServerTls serial number: %s", err.Error())
	}

	return serialNumber, nil
}
