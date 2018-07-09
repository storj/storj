// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto/x509"
	"math/big"
)

func clientTemplate(t *TLSFileOptions) (*x509.Certificate, error) {
	template := &x509.Certificate{
		SerialNumber:          new(big.Int).SetInt64(4),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA: false,
	}

	setHosts(t.Hosts, template)

	return template, nil
}

func rootTemplate(t *TLSFileOptions) (*x509.Certificate, error) {
	serialNumber, err := newSerialNumber()
	if err != nil {
		return nil, ErrTLSTemplate.Wrap(err)
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA: true,
	}

	setHosts(t.Hosts, template)

	return template, nil
}

func leafTemplate(t *TLSFileOptions) (*x509.Certificate, error) {
	serialNumber, err := newSerialNumber()
	if err != nil {
		return nil, ErrTLSTemplate.Wrap(err)
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA: false,
	}

	setHosts(t.Hosts, template)

	return template, nil
}
