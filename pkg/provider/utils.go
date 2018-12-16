// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/storj"
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
	// ErrZeroBytes is returned for zero slice
	ErrZeroBytes = errs.New("byte slice was unexpectedly empty")
)

func decodePEM(PEMBytes []byte) ([][]byte, error) {
	DERBytes := [][]byte{}

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

func newCAWorker(ctx context.Context, difficulty uint16, parentCert *x509.Certificate, parentKey crypto.PrivateKey, caC chan FullCertificateAuthority, eC chan error) {
	var (
		k   crypto.PrivateKey
		i   storj.NodeID
		err error
	)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			k, err = peertls.NewKey()
			if err != nil {
				eC <- err
				return
			}
			switch kE := k.(type) {
			case *ecdsa.PrivateKey:
				i, err = NodeIDFromKey(&kE.PublicKey)
				if err != nil {
					eC <- err
					return
				}
			default:
				eC <- peertls.ErrUnsupportedKey.New("%T", k)
				return
			}
		}

		d, err := i.Difficulty()
		if err != nil {
			eC <- err
			continue
		}
		if d >= difficulty {
			break
		}
	}

	ct, err := peertls.CATemplate()
	if err != nil {
		eC <- err
		return
	}

	c, err := peertls.NewCert(k, parentKey, ct, parentCert)
	if err != nil {
		eC <- err
		return
	}

	ca := FullCertificateAuthority{
		Cert: c,
		Key:  k,
		ID:   i,
	}
	if parentCert != nil {
		ca.RestChain = []*x509.Certificate{parentCert}
	}
	caC <- ca
}

// writeCertData writes data to path ensuring permissions are appropriate for a cert
func writeCertData(path string, data []byte) error {
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
	hasCert := os.IsExist(err)

	_, err = os.Stat(keyPath)
	hasKey := os.IsExist(err)

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
