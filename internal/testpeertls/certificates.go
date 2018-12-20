// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testpeertls

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"math/big"
)

// DebugCert is a subset of the most relevant fields from an x509.Certificate for debugging
type DebugCert struct {
	Raw               []byte
	RawTBSCertificate []byte
	Signature         []byte
	PublicKeyX        *big.Int
	PublicKeyY        *big.Int
	Extensions        []pkix.Extension
}

// NewCertDebug converts an *x509.Certificate into a DebugCert
func NewCertDebug(cert x509.Certificate) DebugCert {
	pubKey := cert.PublicKey.(*ecdsa.PublicKey)
	c := DebugCert{
		Raw:               make([]byte, len(cert.Raw)),
		RawTBSCertificate: make([]byte, len(cert.RawTBSCertificate)),
		Signature:         make([]byte, len(cert.Signature)),
		PublicKeyX:        pubKey.X,
		PublicKeyY:        pubKey.Y,
		Extensions:        []pkix.Extension{},
	}

	copy(c.Raw, cert.Raw)
	copy(c.RawTBSCertificate, cert.RawTBSCertificate)
	copy(c.Signature, cert.Signature)
	for _, e := range cert.ExtraExtensions {
		ext := pkix.Extension{Id: e.Id, Value: make([]byte, len(e.Value))}
		copy(ext.Value, e.Value)
		c.Extensions = append(c.Extensions, ext)
	}

	return c
}

// Cmp is used to compare 2 DebugCerts against each other and print the diff
func (c DebugCert) Cmp(c2 DebugCert, label string) {
	fmt.Println("diff " + label + " ---================================================================---")
	cmpBytes := func(a, b []byte) {
		PrintJSON(bytes.Compare(a, b), "")
	}

	cmpBytes(c.Raw, c2.Raw)
	cmpBytes(c.RawTBSCertificate, c2.RawTBSCertificate)
	cmpBytes(c.Signature, c2.Signature)
	c.PublicKeyX.Cmp(c2.PublicKeyX)
	c.PublicKeyY.Cmp(c2.PublicKeyY)
}

// PrintJSON uses a json marshaler to pretty-print arbitrary data for debugging
// with special considerations for certain, specific types
func PrintJSON(data interface{}, label string) {
	var (
		jsonBytes []byte
		err       error
	)

	switch d := data.(type) {
	case x509.Certificate:
		data = NewCertDebug(d)
	case *x509.Certificate:
		data = NewCertDebug(*d)
	case ecdsa.PublicKey:
		data = struct {
			X *big.Int
			Y *big.Int
		}{
			d.X, d.Y,
		}
	case *ecdsa.PrivateKey:
		data = struct {
			X *big.Int
			Y *big.Int
			D *big.Int
		}{
			d.X, d.Y, d.D,
		}
	}

	jsonBytes, err = json.MarshalIndent(data, "", "\t\t")

	if label != "" {
		fmt.Println(label + ": ---================================================================---")
	}
	if err != nil {
		fmt.Printf("ERROR: %s", err.Error())
	}

	fmt.Println(string(jsonBytes))
	fmt.Println("")
}
