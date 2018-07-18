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

func generateTLS() (tls.Certificate, *ecdsa.PrivateKey, error) {
	var fail = func(err error) (tls.Certificate, *ecdsa.PrivateKey, error) {
		return tls.Certificate{}, nil, err
	}

	rootKey, err := ecdsa.GenerateKey(authECCurve, rand.Reader)
	if err != nil {
		return fail(ErrGenerate.New("failed to generateServerTLS root private key", err))
	}

	rootT, err := rootTemplate()
	if err != nil {
		return fail(ErrGenerate.Wrap(err))
	}

	rootC, err := createCert(
		rootT,
		rootT,
		nil,
		&rootKey.PublicKey,
		rootKey,
		rootKey,
	)
	if err != nil {
		return fail(ErrGenerate.Wrap(err))
	}

	newKey, err := ecdsa.GenerateKey(authECCurve, rand.Reader)
	if err != nil {
		return fail(ErrGenerate.New("failed to generateTLS client private key", err))
	}

	leafT, err := leafTemplate()
	if err != nil {
		return fail(ErrGenerate.Wrap(err))
	}

	leafC, err := createCert(
		leafT,
		rootT,
		rootC.Certificate,
		&newKey.PublicKey,
		rootKey,
		newKey,
	)

	if err != nil {
		return fail(ErrGenerate.Wrap(err))
	}

	return leafC, rootKey, nil
}

func createCert(
	template,
	parentTemplate *x509.Certificate,
	parentDERCerts [][]byte,
	pubKey *ecdsa.PublicKey,
	rootKey,
	privKey *ecdsa.PrivateKey) (tls.Certificate, error) {
	var fail = func(err error) (tls.Certificate, error) { return tls.Certificate{}, err }

	certDERBytes, err := x509.CreateCertificate(rand.Reader, template, parentTemplate, pubKey, rootKey)
	if err != nil {
		return fail(errs.Wrap(err))
	}

	parsedLeaf, _ := x509.ParseCertificate(certDERBytes)

	DERCerts := [][]byte{}
	DERCerts = append(DERCerts, certDERBytes)
	DERCerts = append(DERCerts, parentDERCerts...)

	keyDERBytes, err := KeyToDERBytes(privKey)
	if err != nil {
		return fail(err)
	}

	cert := tls.Certificate{}
	cert.Leaf = parsedLeaf
	cert.Certificate = DERCerts
	cert.PrivateKey, err = x509.ParseECPrivateKey(keyDERBytes)
	if err != nil {
		return fail(errs.New("unable to parse EC private key", err))
	}

	return cert, nil
}

func newSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errs.New("failed to generateServerTls serial number: %s", err.Error())
	}

	return serialNumber, nil
}
