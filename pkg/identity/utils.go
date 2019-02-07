// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pkcrypto"
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
		case pkcrypto.BlockTypeCertificate:
			encChain.AddCert(pemBlock.Bytes)
		case pkcrypto.BlockTypeExtension:
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

// writeChainData writes data to path ensuring permissions are appropriate for a cert
func writeChainData(path string, data []byte) error {
	err := writeFile(path, 0744, 0644, data)
	if err != nil {
		return errs.New("unable to write certificate to \"%s\": %v", path, err)
	}
	return nil
}

// writeKeyData writes data to path ensuring permissions are appropriate for a cert
func writeKeyData(path string, data []byte) error {
	err := writeFile(path, 0700, 0600, data)
	if err != nil {
		return errs.New("unable to write key to \"%s\": %v", path, err)
	}
	return nil
}

// writeFile writes to path, creating directories and files with the necessary permissions
func writeFile(path string, dirmode, filemode os.FileMode, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), dirmode); err != nil {
		return errs.Wrap(err)
	}

	if err := ioutil.WriteFile(path, data, filemode); err != nil {
		return errs.Wrap(err)
	}

	return nil
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
