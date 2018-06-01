package utils

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// (see https://github.com/golang/go/blob/master/LICENSE)

import (
  "crypto/ecdsa"
  "crypto/rand"
  "crypto/x509"
  "crypto/x509/pkix"
  "crypto/elliptic"
  "crypto/tls"
  "bytes"
  "encoding/pem"
  "math/big"
  "net"
  "os"
  "strings"
  "time"
  // "flag"

  "github.com/zeebo/errs"
)

var (
  validFrom = ""                //flag.String("start-date", "", "Creation date formatted as Jan 1 15:04:05 2011")
  validFor  = 365*24*time.Hour  //flag.Duration("duration", 365*24*time.Hour, "Duration that certificate is valid for")
  isCA      = false             //flag.Bool("ca", false, "whether this cert should be its own Certificate Authority")
)

func pemBlockForKey(key *ecdsa.PrivateKey) (_ *pem.Block, _ error) {
  b, err := x509.MarshalECPrivateKey(key)
  if err != nil {
    return nil, errs.New("Unable to marshal ECDSA private key: %v", err.Error())
  }

  return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}, nil
}

func (t *TLSFileOptions) generate() (cert *tls.Certificate, _ error) {
  if t.Hosts == "" {
    return nil, ErrGenerate.Wrap(ErrBadHost.New("no host provided"))
  }

  if err := t.EnsureAbsPaths(); err != nil {
    return nil, err
  }

  privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
  if err != nil {
    return nil, ErrGenerate.Wrap(errs.New("failed to generate private key: %s", err.Error()))
  }

  var notBefore time.Time

  // TODO: `validFrom`
  if len(validFrom) == 0 {
    notBefore = time.Now()
  } else {
    notBefore, err = time.Parse("Jan 2 15:04:05 2006", validFrom)
    if err != nil {
      return nil, ErrGenerate.Wrap(errs.New("Failed to parse creation date: %s\n", err.Error()))
    }
  }

  // TODO: `validFor`
  notAfter := notBefore.Add(validFor)

  serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
  serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
  if err != nil {
    return nil, ErrGenerate.Wrap(errs.New("failed to generate serial number: %s", err.Error()))
  }

  template := x509.Certificate{
    SerialNumber: serialNumber,
    Subject: pkix.Name{
      Organization: []string{"Acme Co"},
    },
    NotBefore: notBefore,
    NotAfter:  notAfter,

    KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
    ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
    BasicConstraintsValid: true,
  }

  hosts := strings.Split(t.Hosts, ",")
  for _, h := range hosts {
    if ip := net.ParseIP(h); ip != nil {
      template.IPAddresses = append(template.IPAddresses, ip)
    } else {
      template.DNSNames = append(template.DNSNames, h)
    }
  }

  // TODO: `isCA`
  if isCA {
    template.IsCA = true
    template.KeyUsage |= x509.KeyUsageCertSign
  }

  // DER encoded
  certDerBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
  if err != nil {
    return nil, ErrGenerate.Wrap(err)
  }

  certFile, err := os.Create(t.CertAbsPath)
  if err != nil {
    return nil, ErrGenerate.Wrap(errs.New("failed to open cert.pem for writing: %s", err.Error()))
  }

  certPemBlock := pem.Block{Type: "CERTIFICATE", Bytes: certDerBytes}

  pem.Encode(certFile, &certPemBlock)
  certFile.Close()

  keyOut, err := os.OpenFile(t.KeyAbsPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
  if err != nil {
    return nil, ErrGenerate.Wrap(errs.New("failed to open key.pem for writing:", err))
  }

  keyPemBlock, err := pemBlockForKey(privKey)
  if err != nil {
    return nil, ErrGenerate.Wrap(err)
  }

  pem.Encode(keyOut, keyPemBlock)
  keyOut.Close()

  certBuffer := bytes.NewBuffer([]byte{})
  pem.Encode(certBuffer, &certPemBlock)

  keyBuffer := bytes.NewBuffer([]byte{})
  pem.Encode(keyBuffer, keyPemBlock)

  certificate, err := tls.X509KeyPair(certBuffer.Bytes(), keyBuffer.Bytes())
  if err != nil {
    return nil, ErrGenerate.Wrap(err)
  }
  return &certificate, nil
}
