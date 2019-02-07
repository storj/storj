// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package debugging

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/nsf/jsondiff"
	"github.com/spacemonkeygo/openssl"

	"storj.io/fork/crypto/x509"
)

var (
	diffOpts = jsondiff.DefaultConsoleOptions()
)

// DebugCert is a subset of the most relevant fields from an x509.Certificate for debugging
type DebugCert struct {
	Cert *x509.Certificate
}

// NewDebugCert converts an *x509.Certificate into a DebugCert
func NewDebugCert(cert x509.Certificate) DebugCert {
	return DebugCert{
		Cert: &cert,
	}
}

func AsSimpleStruct(data interface{}) interface{} {
	switch d := data.(type) {
	case *x509.Certificate:
		return AsSimpleStruct(*d)
	case x509.Certificate:
		return NewDebugCert(d)
	case *ecdsa.PublicKey:
		return AsSimpleStruct(*d)
	case ecdsa.PublicKey:
		return struct {
			X *big.Int
			Y *big.Int
		}{
			d.X, d.Y,
		}
	case *ecdsa.PrivateKey:
		return AsSimpleStruct(*d)
	case ecdsa.PrivateKey:
		return struct {
			X *big.Int
			Y *big.Int
			D *big.Int
		}{
			d.X, d.Y, d.D,
		}
	case *rsa.PublicKey:
		return AsSimpleStruct(*d)
	case rsa.PublicKey:
		return struct {
			N *big.Int
			E int
		}{
			d.N, d.E,
		}
	case *rsa.PrivateKey:
		return AsSimpleStruct(*d)
	case rsa.PrivateKey:
		return struct {
			N      *big.Int
			E      int
			D      *big.Int
			Primes []*big.Int
		}{
			d.N, d.E, d.D, d.Primes,
		}
	case openssl.Key:
		var err error
		data, err = d.AsStruct()
		if err != nil {
			return fmt.Errorf("failed to represent OpenSSL key as struct: %v", err)
		}
		return data
	}
	return data
}

// PrintJSON uses a json marshaler to pretty-print arbitrary data for debugging
// with special considerations for certain, specific types
func PrintJSON(data interface{}, label string) {
	var (
		jsonBytes []byte
		err       error
	)

	jsonBytes, err = json.MarshalIndent(AsSimpleStruct(data), "", "\t\t")

	if label != "" {
		fmt.Println(label + ": ---================================================================---")
	}
	if err != nil {
		fmt.Printf("ERROR: %s", err.Error())
	}

	fmt.Println(string(jsonBytes))
	fmt.Println("")
}

// Cmp is used to compare 2 DebugCerts against each other and print the diff
func (c DebugCert) Cmp(c2 DebugCert, label string) error {
	fmt.Println("diff " + label + " ---================================================================---")
	cJSON, err := c.JSON()
	if err != nil {
		return err
	}

	c2JSON, err := c2.JSON()
	if err != nil {
		return err
	}

	diffType, diff := jsondiff.Compare(cJSON, c2JSON, &diffOpts)
	fmt.Printf("Difference type: %s\n======\n%s", diffType, diff)
	return nil
}

// JSON serializes the certificate to JSON
func (c DebugCert) JSON() ([]byte, error) {
	return json.Marshal(c.Cert)
}
