// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/utils"
)

// TLSFilesStatus is the status of keys
type TLSFilesStatus int

// Four possible outcomes for four files
const (
	NoCertNoKey = TLSFilesStatus(iota)
	CertNoKey
	NoCertKey
	CertKey
)

var (
	// ErrChainLength is used when the length of a cert chain isn't what was expected
	ErrChainLength = errs.Class("cert chain length error")
	// ErrZeroBytes is returned for zero slice
	ErrZeroBytes = errs.New("byte slice was unexpectedly empty")
)

type encodedChain struct {
	chain      [][]byte
	extensions [][][]byte
}

// DecodeAndParseChainPEM parses a PEM chain
func DecodeAndParseChainPEM(PEMBytes []byte) ([]*x509.Certificate, error) {
	var (
		encChain  encodedChain
		blockErrs utils.ErrorGroup
	)
	for {
		var pemBlock *pem.Block
		pemBlock, PEMBytes = pem.Decode(PEMBytes)
		if pemBlock == nil {
			break
		}
		switch pemBlock.Type {
		case peertls.BlockTypeCertificate:
			encChain.AddCert(pemBlock.Bytes)
		case peertls.BlockTypeExtension:
			if err := encChain.AddExtension(pemBlock.Bytes); err != nil {
				blockErrs.Add(err)
			}
		}
	}
	if err := blockErrs.Finish(); err != nil {
		return nil, err
	}

	return encChain.Parse()
}

func decodePEM(PEMBytes []byte) ([][]byte, error) {
	var DERBytes [][]byte

	for {
		var DERBlock *pem.Block

		DERBlock, PEMBytes = pem.Decode(PEMBytes)
		if DERBlock == nil {
			break
		}

		DERBytes = append(DERBytes, DERBlock.Bytes)
	}

	if len(DERBytes) == 0 || len(DERBytes[0]) == 0 {
		return nil, ErrZeroBytes
	}

	return DERBytes, nil
}

func statTLSFiles(certPath, keyPath string) TLSFilesStatus {
	_, err := os.Stat(certPath)
	hasCert := !os.IsNotExist(err)

	_, err = os.Stat(keyPath)
	hasKey := !os.IsNotExist(err)

	if hasCert && hasKey {
		return CertKey
	} else if hasCert {
		return CertNoKey
	} else if hasKey {
		return NoCertKey
	}

	return NoCertNoKey
}

func (t TLSFilesStatus) String() string {
	switch t {
	case CertKey:
		return "certificate and key"
	case CertNoKey:
		return "certificate"
	case NoCertKey:
		return "key"
	}
	return ""
}

func (e *encodedChain) AddCert(b []byte) {
	e.chain = append(e.chain, b)
	e.extensions = append(e.extensions, [][]byte{})
}

func (e *encodedChain) AddExtension(b []byte) error {
	chainLen := len(e.chain)
	if chainLen < 1 {
		return ErrChainLength.New("expected: >= 1; actual: %d", chainLen)
	}

	i := chainLen - 1
	e.extensions[i] = append(e.extensions[i], b)
	return nil
}

func (e *encodedChain) Parse() ([]*x509.Certificate, error) {
	chain, err := ParseCertChain(e.chain)
	if err != nil {
		return nil, err
	}

	var extErrs utils.ErrorGroup
	for i, cert := range chain {
		for _, ee := range e.extensions[i] {
			ext := pkix.Extension{}
			_, err := asn1.Unmarshal(ee, &ext)
			if err != nil {
				extErrs.Add(err)
			}
			cert.ExtraExtensions = append(cert.ExtraExtensions, ext)
		}
	}
	if err := extErrs.Finish(); err != nil {
		return nil, err
	}

	return chain, nil
}
