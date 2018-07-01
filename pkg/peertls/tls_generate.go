// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// (see https://github.com/golang/go/blob/master/LICENSE)

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
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/zeebo/errs"
)

type fileRole int

// type certRole int
// type templateFunc func(*TLSFileOptions) (*x509.Certificate, error)

const (
	// root   certRole = iota
	// leaf
	// client

	rootCert fileRole = iota
	rootKey
	leafCert
	leafKey
	clientCert
	clientKey
)

var (
	// roles = map[certRole]string{
	// 	root:   "root",
	// 	leaf:   "leaf",
	// 	client: "client",
	// }
	//
	// roleTemplates = map[certRole]templateFunc{
	// 	root:   rootTemplate,
	// 	leaf:   leafTemplate,
	// 	client: clientTemplate,
	// }

	fileLabels = map[fileRole]string{
		rootCert:   "root certificate",
		rootKey:    "root key",
		leafCert:   "leaf certificate",
		leafKey:    "leaf key",
		clientCert: "client certificate",
		clientKey:  "client key",
	}

	validFrom = ""                   // flag.String("start-date", "", "Creation date formatted as Jan 1 15:04:05 2011")
	validFor  = 365 * 24 * time.Hour // flag.Duration("duration", 365*24*time.Hour, "Duration that certificate is valid for")
	isCA      = false                // flag.Bool("ca", false, "whether this cert should be its own Certificate Authority")
	name      = pkix.Name{
		Organization: []string{"Storj"},
	}
)

func (t *TLSFileOptions) generateTLS() (_ error) {
	var (
		err error
	)

	if t.Hosts == "" {
		return ErrGenerate.Wrap(ErrBadHost.New("no host provided"))
	}

	if err := t.EnsureAbsPaths(); err != nil {
		return ErrGenerate.Wrap(err)
	}

	// roles := map[certRole]
	// for role, templateFunc := range roleTemplates {
	// 	rivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	// 	if err != nil {
	// 		return nil, ErrGenerate.New("failed to generateServerTLS %s private key", roles[role], err)
	// 	}
	//
	// 	template, err := templateFunc(t)
	// 	if err != nil {
	// 		return nil, ErrGenerate.Wrap(err)
	// 	}
	// }

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return ErrGenerate.New("failed to generateServerTLS root private key", err)
	}

	rootT, err := rootTemplate(t)
	if err != nil {
		return ErrGenerate.Wrap(err)
	}

	_, err = createAndPersist(
		t.RootCertAbsPath,
		t.RootKeyAbsPath,
		rootT,
		rootT,
		&rootKey.PublicKey,
		rootKey,
	)
	if err != nil {
		return ErrGenerate.Wrap(err)
	}

	newKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return ErrGenerate.New("failed to generateTLS client private key", err)
	}

	if t.Client {
		clientT, err := clientTemplate(t)
		if err != nil {
			return ErrGenerate.Wrap(err)
		}

		clientC, err := createAndPersist(
			t.ClientCertAbsPath,
			t.ClientKeyAbsPath,
			clientT,
			rootT,
			&newKey.PublicKey,
			newKey,
		)

		if err != nil {
			return ErrGenerate.Wrap(err)
		}
		// clientC.PrivateKey = newKey

		// clientDERBytes, err := x509.CreateCertificate(rand.Reader, clientT, rootT, &newKey.PublicKey, rootKey)
		// if err != nil {
		// 	return ErrGenerate.Wrap(err)
		// }
		//
		// clientCertBlock := newCertBlock(clientDERBytes)
		// if err != nil {
		// 	return ErrGenerate.Wrap(err)
		// }

		t.ClientCertificate = clientC
	} else {
		leafT, err := leafTemplate(t)
		if err != nil {
			return ErrGenerate.Wrap(err)
		}

		leafC, err := createAndPersist(
			t.LeafCertAbsPath,
			t.LeafKeyAbsPath,
			leafT,
			rootT,
			&newKey.PublicKey,
			newKey,
		)

		if err != nil {
			return ErrGenerate.Wrap(err)
		}
		// leafC.PrivateKey = newKey

		t.LeafCertificate = leafC
	}

	return nil
}

func setHosts(hosts string, template *x509.Certificate) {
	h := strings.Split(hosts, ",")
	for _, host := range h {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}
}

func clientTemplate(t *TLSFileOptions) (_ *x509.Certificate, _ error) {
	notBefore, notAfter, err := datesValid()
	if err != nil {
		return nil, ErrTLSTemplate.Wrap(err)
	}

	template := &x509.Certificate{
		SerialNumber:          new(big.Int).SetInt64(4),
		Subject:               name,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA: false,
	}

	setHosts(t.Hosts, template)

	return template, nil
}

// func serverTemplate(t *TLSFileOptions) (_ *x509.Certificate, _ error) {
// 	notBefore, notAfter, err := datesValid()
// 	if err != nil {
// 		return nil, ErrTLSTemplate.Wrap(err)
// 	}
//
// 	serialNumber, err := newSerialNumber()
// 	if err != nil {
// 		return nil, ErrTLSTemplate.Wrap(err)
// 	}
//
// 	template := &x509.Certificate{
// 		SerialNumber:          serialNumber,
// 		Subject:               name,
// 		NotBefore:             notBefore,
// 		NotAfter:              notAfter,
// 		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
// 		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
// 		BasicConstraintsValid: true,
// 	}
//
// 	// TODO: `isCA`
// 	if isCA {
// 		template.IsCA = true
// 		template.KeyUsage |= x509.KeyUsageCertSign
// 	}
//
// 	setHosts(t.Hosts, template)
//
// 	return template, nil
// }

func rootTemplate(t *TLSFileOptions) (_ *x509.Certificate, _ error) {
	notBefore, notAfter, err := datesValid()
	if err != nil {
		return nil, ErrTLSTemplate.Wrap(err)
	}

	serialNumber, err := newSerialNumber()
	if err != nil {
		return nil, ErrTLSTemplate.Wrap(err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
			CommonName:   "Root CA",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA: true,
	}

	setHosts(t.Hosts, template)

	return template, nil
}

func leafTemplate(t *TLSFileOptions) (_ *x509.Certificate, _ error) {
	notBefore, notAfter, err := datesValid()
	if err != nil {
		return nil, ErrTLSTemplate.Wrap(err)
	}

	serialNumber, err := newSerialNumber()
	if err != nil {
		return nil, ErrTLSTemplate.Wrap(err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
			CommonName:   "test_cert_1",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA: false,
	}

	setHosts(t.Hosts, template)

	return template, nil
}

func newKeyBlock(b []byte) (_ *pem.Block) {
	return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
}

func newCertBlock(b []byte) (_ *pem.Block) {
	return &pem.Block{Type: "CERTIFICATE", Bytes: b}
}

func keyToDERBytes(key *ecdsa.PrivateKey) (_ []byte, _ error) {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, errs.New("unable to marshal ECDSA private key", err)
	}

	return b, nil
}

func keyToBlock(key *ecdsa.PrivateKey) (_ *pem.Block, _ error) {
	b, err := keyToDERBytes(key)
	if err != nil {
		return nil, err
	}

	return newKeyBlock(b), nil
}

func writePem(block *pem.Block, file io.WriteCloser) (_ error) {
	if err := pem.Encode(file, block); err != nil {
		return errs.New("unable to PEM-encode/write bytes to file", err)
	}

	return nil
}

func writeCert(b []byte, path string) (_ error) {
	file, err := os.Create(path)
	defer file.Close()

	if err != nil {
		return errs.New("unable to open file \"%s\" for writing", path, err)
	}

	block := newCertBlock(b)
	if err := writePem(block, file); err != nil {
		return err
	}

	if err := file.Close(); err != nil {
		return errs.New("unable to close file", err)
	}

	return nil
}

func writeKey(key *ecdsa.PrivateKey, path string) (_ error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errs.New("unable to open \"%s\" for writing", path, err)
	}

	block, err := keyToBlock(key)
	if err != nil {
		return err
	}

	if err := writePem(block, file); err != nil {
		return err
	}

	return nil
}

// LoadX509KeyPair reads and parses a public/private key pair from a pair
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

	certPEMBlock, _ := pem.Decode(certPEMBytes)
	keyPEMBlock, _ := pem.Decode(keyPEMBytes)
	return certFromPEMs(certPEMBlock.Bytes, keyPEMBlock.Bytes)
	// return certFromPEMs(certPEMBlock, keyPEMBlock)
	// return certFromPEMs(certPEMBytes, keyPEMBytes)
}

// X509KeyPair parses a public/private key pair from a pair of
// PEM encoded data. On successful return, Certificate.Leaf will be nil because
// the parsed form of the certificate is not retained.
func certFromPEMs(certDERBytes, keyDERBytes []byte) (*tls.Certificate, error) {
	fail := func(err error) (*tls.Certificate, error) { return &tls.Certificate{}, err }

	var cert = new(tls.Certificate)
	cert.Certificate = append(cert.Certificate, certDERBytes)

	var err error
	cert.PrivateKey, err = x509.ParseECPrivateKey(keyDERBytes)
	if err != nil {
		return fail(err)
	}

	return cert, nil
}

// func certFromPEMs(cert, key *pem.Block) (_ *tls.Certificate, _ error) {
// 	certBuffer := bytes.NewBuffer([]byte{})
// 	pem.Encode(certBuffer, cert)
//
// 	keyBuffer := bytes.NewBuffer([]byte{})
// 	pem.Encode(keyBuffer, key)
//
// 	certificate, err := tls.X509KeyPair(certBuffer.Bytes(), keyBuffer.Bytes())
// 	if err != nil {
// 		return nil, errs.New("unable to get certificate from PEM-encoded cert/key bytes", err)
// 	}
//
// 	return &certificate, nil
// }

func createAndPersist(certPath, keyPath string, template, parent *x509.Certificate, pubKey *ecdsa.PublicKey, privKey *ecdsa.PrivateKey) (_ *tls.Certificate, _ error) {
	// DER encoded
	certDerBytes, err := x509.CreateCertificate(rand.Reader, template, parent, pubKey, privKey)
	if err != nil {
		return nil, err
	}

	if err := writeCert(certDerBytes, certPath); err != nil {
		return nil, err
	}

	if err := writeKey(privKey, keyPath); err != nil {
		return nil, err
	}

	keyDERBytes, err := keyToDERBytes(privKey)
	if err != nil {
		return nil, err
	}

	return certFromPEMs(certDerBytes, keyDERBytes)

}

func datesValid() (_, _ time.Time, _ error) {
	var notBefore time.Time

	// TODO: `validFrom`
	if len(validFrom) == 0 {
		notBefore = time.Now()
	} else {
		var err error
		notBefore, err = time.Parse("Jan 2 15:04:05 2006", validFrom)
		if err != nil {
			return time.Time{}, time.Time{}, errs.New("failed to parse creation date", err)
		}
	}

	// TODO: `validFor`
	notAfter := notBefore.Add(validFor)

	return notBefore, notAfter, nil
}

func newSerialNumber() (_ *big.Int, _ error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errs.New("failed to generateServerTls serial number: %s", err.Error())
	}

	return serialNumber, nil
}
