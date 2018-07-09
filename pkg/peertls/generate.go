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
	"io/ioutil"
	"math/big"
	"time"

	"github.com/zeebo/errs"
)

const (
	OneYear = 365 * 24 * time.Hour
)

func (t *TLSFileOptions) generateTLS() error {
	if err := t.EnsureAbsPaths(); err != nil {
		return ErrGenerate.Wrap(err)
	}

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return ErrGenerate.New("failed to generateServerTLS root private key", err)
	}

	rootT, err := rootTemplate(t)
	if err != nil {
		return ErrGenerate.Wrap(err)
	}

	rootC, err := createAndWrite(
		t.RootCertAbsPath,
		t.RootKeyAbsPath,
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

	leafC, err := createAndWrite(
		t.LeafCertAbsPath,
		t.LeafKeyAbsPath,
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

	t.LeafCertificate = leafC

	return nil
}

// LoadCert reads and parses a cert/privkey pair from a pair
// of files. The files must contain PEM encoded data. The certificate file
// may contain intermediate certificates following the leaf certificate to
// form a certificate chain. On successful return, Certificate.Leaf will
// be nil because the parsed form of the certificate is not retained.
func LoadCert(certFile, keyFile string) (*tls.Certificate, error) {
	certPEMBytes, err := ioutil.ReadFile(certFile)
	if err != nil {
		return &tls.Certificate{}, err
	}
	keyPEMBytes, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return &tls.Certificate{}, err
	}

	return certFromPEMs(certPEMBytes, keyPEMBytes)
}

func createAndWrite(
	certPath,
	keyPath string,
	template,
	parentTemplate *x509.Certificate,
	parentDERCerts [][]byte,
	pubKey *ecdsa.PublicKey,
	rootKey,
	privKey *ecdsa.PrivateKey) (*tls.Certificate, error) {

	DERCerts, keyDERBytes, err := createDERs(
		template,
		parentTemplate,
		parentDERCerts,
		pubKey,
		rootKey,
		privKey,
	)
	if err != nil {
		return nil, err
	}

	if err := writeCerts(DERCerts, certPath); err != nil {
		return nil, err
	}

	if err := writeKey(privKey, keyPath); err != nil {
		return nil, err
	}

	return certFromDERs(DERCerts, keyDERBytes)
}

func createDERs(
	template,
	parentTemplate *x509.Certificate,
	parentDERCerts [][]byte,
	pubKey *ecdsa.PublicKey,
	rootKey,
	privKey *ecdsa.PrivateKey) (_ [][]byte, _ []byte, _ error) {
	certDERBytes, err := x509.CreateCertificate(rand.Reader, template, parentTemplate, pubKey, rootKey)
	if err != nil {
		return nil, nil, err
	}

	DERCerts := [][]byte{}
	DERCerts = append(DERCerts, certDERBytes)
	DERCerts = append(DERCerts, parentDERCerts...)

	keyDERBytes, err := keyToDERBytes(privKey)
	if err != nil {
		return nil, nil, err
	}

	return DERCerts, keyDERBytes, nil
}

func newSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errs.New("failed to generateServerTls serial number: %s", err.Error())
	}

	return serialNumber, nil
}
