// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/utils"
)

// PeerCertificateAuthority represents the CA which is used to validate peer identities
type PeerCertificateAuthority struct {
	// Cert is the x509 certificate of the CA
	Cert *x509.Certificate
	// The ID is calculated from the CA public key.
	ID nodeID
}

// FullCertificateAuthority represents the CA which is used to author and validate full identities
type FullCertificateAuthority struct {
	// Cert is the x509 certificate of the CA
	Cert *x509.Certificate
	// The ID is calculated from the CA public key.
	ID nodeID
	// Key is the private key of the CA
	Key crypto.PrivateKey
}

type CASetupConfig struct {
	CertPath    string `help:"path to the certificate chain for this identity" default:"$CONFDIR/ca.cert"`
	KeyPath     string `help:"path to the private key for this identity" default:"$CONFDIR/ca.key"`
	Difficulty  uint64 `help:"minimum difficulty for identity generation" default:"24"`
	Timeout     string `help:"timeout for CA generation; golang duration string (0 no timeout)" default:"5m"`
	Overwrite   bool   `help:"if true, existing CA certs AND keys will overwritten" default:"false"`
	Concurrency uint   `help:"number of concurrent workers for certificate authority generation" default:"4"`
}

type CAConfig struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$CONFDIR/ca.cert"`
	KeyPath  string `help:"path to the private key for this identity" default:"$CONFDIR/ca.key"`
}

// Stat returns the status of the CA cert/key files for the config
func (caS CASetupConfig) Stat() TlsFilesStat {
	return statTLSFiles(caS.CertPath, caS.KeyPath)
}

// Create generates and saves a CA using the config
func (caS CASetupConfig) Create(ctx context.Context, concurrency uint) (*FullCertificateAuthority, error) {
	ca, err := GenerateCA(ctx, uint16(caS.Difficulty), concurrency)
	if err != nil {
		return nil, err
	}
	caC := CAConfig{
		CertPath: caS.CertPath,
		KeyPath:  caS.KeyPath,
	}
	return ca, caC.Save(ca)
}

// Load loads a CA from the given configuration
func (caC CAConfig) Load() (*FullCertificateAuthority, error) {
	cd, err := ioutil.ReadFile(caC.CertPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}
	kb, err := ioutil.ReadFile(caC.KeyPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}

	var cb [][]byte
	for {
		var cp *pem.Block
		cp, cd = pem.Decode(cd)
		if cp == nil {
			break
		}
		cb = append(cb, cp.Bytes)
	}
	c, err := ParseCertChain(cb)
	if err != nil {
		return nil, errs.New("failed to load identity %#v, %#v: %v",
			caC.CertPath, caC.KeyPath, err)
	}

	kp, _ := pem.Decode(kb)
	k, err := x509.ParseECPrivateKey(kp.Bytes)
	if err != nil {
		return nil, errs.New("unable to parse EC private key", err)
	}
	i, err := idFromKey(k)
	if err != nil {
		return nil, err
	}

	return &FullCertificateAuthority{
		Cert: c[0],
		Key:  k,
		ID:   i,
	}, nil
}

// GenerateCA creates a new full identity with the given difficulty
func GenerateCA(ctx context.Context, difficulty uint16, concurrency uint) (*FullCertificateAuthority, error) {
	if concurrency < 1 {
		concurrency = 1
	}
	ctx, cancel := context.WithCancel(ctx)

	eC := make(chan error)
	caC := make(chan FullCertificateAuthority, 1)
	for i := 0; i < int(concurrency); i++ {
		go generateCAWorker(ctx, difficulty, caC, eC)
	}

	select {
	case ca := <-caC:
		cancel()
		return &ca, nil
	case err := <-eC:
		cancel()
		return nil, err
	}
}

// Save saves a CA with the given configuration
func (caC CAConfig) Save(ca *FullCertificateAuthority) error {
	f := os.O_WRONLY | os.O_CREATE
	c, err := openCert(caC.CertPath, f)
	if err != nil {
		return err
	}
	defer utils.LogClose(c)
	k, err := openKey(caC.KeyPath, f)
	if err != nil {
		return err
	}
	defer utils.LogClose(k)

	if err = peertls.WriteChain(c, ca.Cert); err != nil {
		return err
	}
	if err = peertls.WriteKey(k, ca.Key); err != nil {
		return err
	}
	return nil
}

// Generate Identity generates a new `FullIdentity` based on the CA. The CA
// cert is included in the identity's cert chain and the identity's leaf cert
// is signed by the CA.
func (ca FullCertificateAuthority) GenerateIdentity() (*FullIdentity, error) {
	lT, err := peertls.LeafTemplate()
	if err != nil {
		return nil, err
	}
	l, err := peertls.NewCert(lT, ca.Cert, ca.Key)
	if err != nil {
		return nil, err
	}
	k, err := peertls.NewKey()
	if err != nil {
		return nil, err
	}

	return &FullIdentity{
		CA:   ca.Cert,
		Leaf: l,
		Key:  k,
		ID:   ca.ID,
	}, nil
}
