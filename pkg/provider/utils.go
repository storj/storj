// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
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

func openCert(path string, flag int) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0744); err != nil {
		return nil, errs.Wrap(err)
	}

	c, err := os.OpenFile(path, flag, 0644)
	if err != nil {
		return nil, errs.New("unable to open cert file for writing \"%s\": %v", path, err)
	}
	return c, nil
}

func openKey(path string, flag int) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, errs.Wrap(err)
	}

	k, err := os.OpenFile(path, flag, 0600)
	if err != nil {
		return nil, errs.New("unable to open key file for writing \"%s\": %v", path, err)
	}
	return k, nil
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
