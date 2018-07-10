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

func (t *TLSHelper) generateTLS() (error) {
	// if err := t.EnsureAbsPaths(); err != nil {
	// 	return ErrGenerate.Wrap(err)
	// }

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return ErrGenerate.New("failed to generateServerTLS root private key", err)
	}

	rootT, err := rootTemplate(t)
	if err != nil {
		return ErrGenerate.Wrap(err)
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
		return ErrGenerate.Wrap(err)
	}

	newKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return ErrGenerate.New("failed to generateTLS client private key", err)
	}

	leafT, err := leafTemplate(t)
	if err != nil {
		return ErrGenerate.Wrap(err)
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
		return ErrGenerate.Wrap(err)
	}

	t.cert = leafC

	return nil
}

// func createAndWrite(
// 		certPath,
// 		keyPath string,
// 		template,
// 		parentTemplate *x509.Certificate,
// 		parentDERCerts [][]byte,
// 		pubKey *ecdsa.PublicKey,
// 		rootKey,
// 		privKey *ecdsa.PrivateKey) (*tls.Certificate, error) {
//
// 	DERCerts, keyDERBytes, err := createDERs(
// 		template,
// 		parentTemplate,
// 		parentDERCerts,
// 		pubKey,
// 		rootKey,
// 		privKey,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	if err := writeCerts(DERCerts, certPath); err != nil {
// 		return nil, err
// 	}
//
// 	if err := writeKey(privKey, keyPath); err != nil {
// 		return nil, err
// 	}
//
// 	return certFromDERs(DERCerts, keyDERBytes)
//
// }

func createCert(
		template,
		parentTemplate *x509.Certificate,
		parentDERCerts [][]byte,
		pubKey *ecdsa.PublicKey,
		rootKey,
		privKey *ecdsa.PrivateKey) (*tls.Certificate, error) {
	certDERBytes, err := x509.CreateCertificate(rand.Reader, template, parentTemplate, pubKey, rootKey)
	if err != nil {
		return nil, err
	}

	DERCerts := [][]byte{}
	DERCerts = append(DERCerts, certDERBytes)
	DERCerts = append(DERCerts, parentDERCerts...)

	keyDERBytes, err := keyToDERBytes(privKey)
	if err != nil {
		return nil, err
	}

	cert := new(tls.Certificate)
	cert.Certificate = DERCerts
	cert.PrivateKey, err = x509.ParseECPrivateKey(keyDERBytes)
	if err != nil {
		return nil, errs.New("unable to parse EC private key", err)
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
