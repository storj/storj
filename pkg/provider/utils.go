// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"golang.org/x/crypto/sha3"

	"storj.io/storj/pkg/peertls"
)

type TlsFilesStat int

const (
	NoCertNoKey = iota
	CertNoKey
	NoCertKey
	CertKey
)

var (
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

func generateCAWorker(ctx context.Context, difficulty uint16, caC chan FullCertificateAuthority, eC chan error) {
	var (
		k   crypto.PrivateKey
		i   nodeID
		err error
	)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			k, err = peertls.NewKey()
			switch kE := k.(type) {
			case *ecdsa.PrivateKey:
				i, err = idFromKey(&kE.PublicKey)
				if err != nil {
					eC <- err
					return
				}
			default:
				eC <- peertls.ErrUnsupportedKey.New("%T", k)
				return
			}
		}

		if i.Difficulty() >= difficulty {
			break
		}
	}

	ct, err := peertls.CATemplate()
	if err != nil {
		eC <- err
		return
	}

	c, err := peertls.NewCert(ct, nil, k)
	if err != nil {
		eC <- err
		return
	}

	ca := FullCertificateAuthority{
		Cert: c,
		Key:  k,
		ID:   i,
	}
	caC <- ca
	return
}

func idFromKey(k crypto.PublicKey) (nodeID, error) {
	kb, err := x509.MarshalPKIXPublicKey(k)
	if err != nil {
		return "", errs.Wrap(err)
	}
	hash := make([]byte, IdentityLength)
	sha3.ShakeSum256(hash, kb)
	return nodeID(base64.URLEncoding.EncodeToString(hash)), nil
}

func openCert(path string, flag int) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 744); err != nil {
		return nil, errs.Wrap(err)
	}

	c, err := os.OpenFile(path, flag, 0644)
	if err != nil {
		return nil, errs.New("unable to open cert file for writing \"%s\"", path, err)
	}
	return c, nil
}

func openKey(path string, flag int) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 700); err != nil {
		return nil, errs.Wrap(err)
	}

	k, err := os.OpenFile(path, flag, 0600)
	if err != nil {
		return nil, errs.New("unable to open key file for writing \"%s\"", path, err)
	}
	return k, nil
}

func statTLSFiles(certPath, keyPath string) TlsFilesStat {
	s := 0
	_, err := os.Stat(certPath)
	if err == nil {
		s += 1
	}
	_, err = os.Stat(keyPath)
	if err == nil {
		s += 2
	}
	return TlsFilesStat(s)
}

func (t TlsFilesStat) String() string {
	switch t {
	case CertKey:
		return "certificate and key"
	case CertNoKey:
		return "certificate"
	case NoCertKey:
		return "key"
	default:
		return ""
	}
}
