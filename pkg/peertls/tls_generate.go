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
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"time"
	// "flag"

	"crypto/elliptic"
	"net"
	"strings"

	"io"
	"github.com/zeebo/errs"
)

var (
	validFrom = ""                   // flag.String("start-date", "", "Creation date formatted as Jan 1 15:04:05 2011")
	validFor  = 365 * 24 * time.Hour // flag.Duration("duration", 365*24*time.Hour, "Duration that certificate is valid for")
	isCA      = false                // flag.Bool("ca", false, "whether this cert should be its own Certificate Authority")
	name      = pkix.Name{
		Organization: []string{"Storj"},
	}
)

func (t *TLSFileOptions) generateServerTls() (_ error) {
	if t.Hosts == "" {
		return ErrGenerate.Wrap(ErrBadHost.New("no host provided"))
	}

	if err := t.EnsureAbsPaths(); err != nil {
		return err
	}

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return ErrGenerate.New("failed to generateServerTls private key", err)
	}

	template, err := serverTemplate()

	if err != nil {
		return ErrGenerate.Wrap(err)
	}

	hosts := strings.Split(t.Hosts, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	// DER encoded
	certDerBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		return ErrGenerate.Wrap(err)
	}

	if err := writeCert(certDerBytes, t.CertAbsPath); err != nil {
		return ErrGenerate.Wrap(err)
	}

	if err := writeKey(privKey, t.KeyAbsPath); err != nil {
		return ErrGenerate.Wrap(err)
	}

	keyPem, err := keyToPem(privKey)
	if err != nil {
		return ErrGenerate.Wrap(err)
	}

	certificate, err := certFromPems(newCertBlock(certDerBytes), keyPem)
	if err != nil {
		return ErrGenerate.Wrap(err)
	}

	t.Certificate = certificate

	return nil
}

func (t *TLSFileOptions) generateClientTls() (_ error) {
	if t.Hosts == "" {
		return ErrGenerate.Wrap(ErrBadHost.New("no host provided"))
	}

	if err := t.EnsureAbsPaths(); err != nil {
		return err
	}

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return ErrGenerate.New("failed to generateServerTls private key", err)
	}

	template, err := clientTemplate()

	if err != nil {
		return ErrGenerate.Wrap(err)
	}

	hosts := strings.Split(t.Hosts, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	// DER encoded
	certDerBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		return ErrGenerate.Wrap(err)
	}

	if err := writeCert(certDerBytes, t.CertAbsPath); err != nil {
		return ErrGenerate.Wrap(err)
	}

	if err := writeKey(privKey, t.KeyAbsPath); err != nil {
		return ErrGenerate.Wrap(err)
	}

	keyPem, err := keyToPem(privKey)
	if err != nil {
		return ErrGenerate.Wrap(err)
	}

	certificate, err := certFromPems(newCertBlock(certDerBytes), keyPem)
	if err != nil {
		return ErrGenerate.Wrap(err)
	}

	t.Certificate = certificate

	return nil
}

func clientTemplate() (_ *x509.Certificate, _ error) {
	var notBefore time.Time

	// TODO: `validFrom`
	if len(validFrom) == 0 {
		notBefore = time.Now()
	} else {
		var err error
		notBefore, err = time.Parse("Jan 2 15:04:05 2006", validFrom)
		if err != nil {
			return nil, ErrTLSTemplate.New("Failed to parse creation date", err)
		}
	}

	// TODO: `validFor`
	notAfter := notBefore.Add(validFor)

	return &x509.Certificate{
		SerialNumber:          new(big.Int).SetInt64(4),
		Subject:               name,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA: false,
	}, nil
}

func serverTemplate() (_ *x509.Certificate, _ error) {
	var notBefore time.Time

	// TODO: `validFrom`
	if len(validFrom) == 0 {
		notBefore = time.Now()
	} else {
		var err error
		notBefore, err = time.Parse("Jan 2 15:04:05 2006", validFrom)
		if err != nil {
			return nil, ErrTLSTemplate.New("Failed to parse creation date", err)
		}
	}

	// TODO: `validFor`
	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, ErrTLSTemplate.New("failed to generateServerTls serial number: %s", err.Error())
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               name,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// TODO: `isCA`
	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	return template, nil
}

func newKeyBlock(b []byte) (_ *pem.Block) {
	return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
}

func newCertBlock(b []byte) (_ *pem.Block) {
	return &pem.Block{Type: "CERTIFICATE", Bytes: b}
}

func keyToPem(key *ecdsa.PrivateKey) (_ *pem.Block, _ error) {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, errs.New("unable to marshal ECDSA private key", err)
	}

	return newKeyBlock(b), nil
}

func writePem(block *pem.Block, file io.WriteCloser) (_ error) {
	if err := pem.Encode(file, block); err != nil {
		return errs.New("unable to PEM-encode/write bytes to file", err)
	}

	if err := file.Close(); err != nil {
		return errs.New("unable to close file", err)
	}

	return nil
}

func writeCert(b []byte, path string) (_ error) {
	file, err := os.Create(path)
	if err != nil {
		return errs.New("unable to open file \"%s\" for writing", path, err)
	}

	block := newCertBlock(b)

	if err := writePem(block, file); err != nil {
		return err
	}

	return nil
}

func writeKey(key *ecdsa.PrivateKey, path string) (_ error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errs.New("unable to open \"%s\" for writing", path, err)
	}

	block, err := keyToPem(key)
	if err != nil {
		return err
	}

	if err := writePem(block, file); err != nil {
		return err
	}

	return nil
}

func certFromPems(cert, key *pem.Block) (_ *tls.Certificate, _ error) {
	certBuffer := bytes.NewBuffer([]byte{})
	pem.Encode(certBuffer, cert)

	keyBuffer := bytes.NewBuffer([]byte{})
	pem.Encode(keyBuffer, key)

	certificate, err := tls.X509KeyPair(certBuffer.Bytes(), keyBuffer.Bytes())
	if err != nil {
		return nil, errs.New("unable to get certificate from PEM-encoded cert/key bytes", err)
	}

	return &certificate, nil
}

// func readPem(path string) (_ *pem.Block, _ error) {
// 	b, err := ioutil.ReadFile(path)
// 	if err != nil {
// 		return nil, errs.New("unable to open key file \"%s\" for reading", path, err)
// 	}
//
// 	// NB: only decodes *first* PEM encoded block?
// 	block, _ := pem.Decode(b)
//
// 	return block, nil
// }

// func readCertificate(certPath, keyPath string) (_ *tls.Certificate, _ error) {
// 	certBlock, err := readPem(certPath)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	keyBlock, err := readPem(keyPath)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return certFromPems(certBlock, keyBlock)
// }
