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

type ecdsaSignature struct {
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
			return nil, ErrVerifyPeerCert.New("unable to parse certificate: %v", err)
		}
	}
	return certs, nil
}

func verifyChainSignatures(certs []*x509.Certificate) error {
	for i, cert := range certs {
		j := len(certs)
		if i+1 < j {
			isValid, err := verifyCertSignature(certs[i+1], cert)
			if err != nil {
				return ErrVerifyPeerCert.Wrap(err)
			}

			if !isValid {
				return ErrVerifyPeerCert.New("certificate chain signature verification failed")
			}

			continue
		}

		rootIsValid, err := verifyCertSignature(cert, cert)
		if err != nil {
			return ErrVerifyPeerCert.Wrap(err)
		}

		if !rootIsValid {
			return ErrVerifyPeerCert.New("certificate chain signature verification failed")
		}
	}

	return nil
}

func verifyCertSignature(parentCert, childCert *x509.Certificate) (bool, error) {
	pubKey, ok := parentCert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return false, ErrUnsupportedKey.New("%T", parentCert.PublicKey)
	}

	signature := new(ecdsaSignature)

	if _, err := asn1.Unmarshal(childCert.Signature, signature); err != nil {
		return false, ErrVerifySignature.New("unable to unmarshal ecdsa signature: %v", err)
	}

	h := crypto.SHA256.New()
	_, err := h.Write(childCert.RawTBSCertificate)
	if err != nil {
		return false, err
	}
	digest := h.Sum(nil)

	isValid := ecdsa.Verify(pubKey, digest, signature.R, signature.S)

	return isValid, nil
}

func newSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errs.New("failed to generateServerTls serial number: %s", err.Error())
	}

	return serialNumber, nil
}
