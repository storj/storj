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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"

	"github.com/zeebo/errs"
)

var authECCurve = elliptic.P256()

func Generate() (leaf, ca *tls.Certificate, _ error) {
	var fail = func(err error) (_, _ *tls.Certificate, _ error) {
		return nil, nil, err
	}

	caKey, err := ecdsa.GenerateKey(authECCurve, rand.Reader)
	if err != nil {
		return fail(ErrGenerate.New("failed to generateServerTLS root private key", err))
	}

	caTemplate, err := rootTemplate()
	if err != nil {
		return fail(ErrGenerate.Wrap(err))
	}

	caCert, err := createCert(
		caTemplate,
		caTemplate,
		nil,
		&caKey.PublicKey,
		caKey,
		caKey,
	)
	if err != nil {
		return fail(ErrGenerate.Wrap(err))
	}

	leafKey, err := ecdsa.GenerateKey(authECCurve, rand.Reader)
	if err != nil {
		return fail(ErrGenerate.New("failed to generateTLS client private key", err))
	}

	leafTemplate, err := leafTemplate()
	if err != nil {
		return fail(ErrGenerate.Wrap(err))
	}

	leafCert, err := createCert(
		leafTemplate,
		caTemplate,
		caCert.Certificate,
		&leafKey.PublicKey,
		caKey,
		leafKey,
	)

	if err != nil {
		return fail(ErrGenerate.Wrap(err))
	}

	return leafCert, caCert, nil
}

func createCert(
	template,
	parentTemplate *x509.Certificate,
	parentDERCerts [][]byte,
	pubKey *ecdsa.PublicKey,
	signingKey,
	privKey *ecdsa.PrivateKey) (*tls.Certificate, error) {

	certDERBytes, err := x509.CreateCertificate(
		rand.Reader,
		template,
		parentTemplate,
		pubKey,
		signingKey,
	)

	if err != nil {
		return nil, errs.Wrap(err)
	}

	parsedLeaf, _ := x509.ParseCertificate(certDERBytes)

	DERCerts := [][]byte{}
	DERCerts = append(DERCerts, certDERBytes)
	DERCerts = append(DERCerts, parentDERCerts...)

	cert := tls.Certificate{}
	cert.Leaf = parsedLeaf
	cert.Certificate = DERCerts
	cert.PrivateKey = privKey

	return &cert, nil
}

func newSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errs.New("failed to generateServerTls serial number: %s", err.Error())
	}

	return serialNumber, nil
}
