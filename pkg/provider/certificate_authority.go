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

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/utils"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/storj"
)

// PeerCertificateAuthority represents the CA which is used to validate peer identities
type PeerCertificateAuthority struct {
	RestChain []*x509.Certificate
	// Cert is the x509 certificate of the CA
	Cert *x509.Certificate
	// The ID is calculated from the CA public key.
	ID storj.NodeID
}

// FullCertificateAuthority represents the CA which is used to author and validate full identities
type FullCertificateAuthority struct {
	RestChain []*x509.Certificate
	// Cert is the x509 certificate of the CA
	Cert *x509.Certificate
	// The ID is calculated from the CA public key.
	ID storj.NodeID
	// Key is the private key of the CA
	Key crypto.PrivateKey
}

// CASetupConfig is for creating a CA
type CASetupConfig struct {
	ParentCertPath string `help:"path to the parent authority's certificate chain"`
	ParentKeyPath  string `help:"path to the parent authority's private key"`
	CertPath       string `help:"path to the certificate chain for this identity" default:"$CONFDIR/ca.cert"`
	KeyPath        string `help:"path to the private key for this identity" default:"$CONFDIR/ca.key"`
	Difficulty     uint64 `help:"minimum difficulty for identity generation" default:"12"`
	Timeout        string `help:"timeout for CA generation; golang duration string (0 no timeout)" default:"5m"`
	Overwrite      bool   `help:"if true, existing CA certs AND keys will overwritten" default:"false"`
	Concurrency    uint   `help:"number of concurrent workers for certificate authority generation" default:"4"`
}

// NewCAOptions is used to pass parameters to `NewCA`
type NewCAOptions struct {
	// Difficulty is the number of trailing zero-bits the nodeID must have
	Difficulty uint16
	// Concurrency is the number of go routines used to generate a CA of sufficient difficulty
	Concurrency uint
	// ParentCert, if provided will be prepended to the certificate chain
	ParentCert *x509.Certificate
	// ParentKey ()
	ParentKey crypto.PrivateKey
}

// PeerCAConfig is for locating a CA certificate without a private key
type PeerCAConfig struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$CONFDIR/ca.cert"`
}

// FullCAConfig is for locating a CA certificate and it's private key
type FullCAConfig struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$CONFDIR/ca.cert"`
	KeyPath  string `help:"path to the private key for this identity" default:"$CONFDIR/ca.key"`
}

// Status returns the status of the CA cert/key files for the config
func (caS CASetupConfig) Status() TLSFilesStatus {
	return statTLSFiles(caS.CertPath, caS.KeyPath)
}

// Create generates and saves a CA using the config
func (caS CASetupConfig) Create(ctx context.Context) (*FullCertificateAuthority, error) {
	var (
		err    error
		parent *FullCertificateAuthority
	)
	if caS.ParentCertPath != "" && caS.ParentKeyPath != "" {
		parent, err = FullCAConfig{
			CertPath: caS.ParentCertPath,
			KeyPath:  caS.ParentKeyPath,
		}.Load()
	}
	if err != nil {
		return nil, err
	}

	if parent == nil {
		parent = &FullCertificateAuthority{}
	}

	ca, err := NewCA(ctx, NewCAOptions{
		Difficulty:  uint16(caS.Difficulty),
		Concurrency: caS.Concurrency,
		ParentCert:  parent.Cert,
		ParentKey:   parent.Key,
	})
	if err != nil {
		return nil, err
	}
	caC := FullCAConfig{
		CertPath: caS.CertPath,
		KeyPath:  caS.KeyPath,
	}
	return ca, caC.Save(ca)
}

// Load loads a CA from the given configuration
func (fc FullCAConfig) Load() (*FullCertificateAuthority, error) {
	p, err := fc.PeerConfig().Load()
	if err != nil {
		return nil, err
	}

	kb, err := ioutil.ReadFile(fc.KeyPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}
	kp, _ := pem.Decode(kb)
	k, err := x509.ParseECPrivateKey(kp.Bytes)
	if err != nil {
		return nil, errs.New("unable to parse EC private key: %v", err)
	}

	return &FullCertificateAuthority{
		RestChain: p.RestChain,
		Cert:      p.Cert,
		Key:       k,
		ID:        p.ID,
	}, nil
}

// PeerConfig converts a full ca config to a peer ca config
func (fc FullCAConfig) PeerConfig() PeerCAConfig {
	return PeerCAConfig{
		CertPath: fc.CertPath,
	}
}

// Load loads a CA from the given configuration
func (pc PeerCAConfig) Load() (*PeerCertificateAuthority, error) {
	cd, err := ioutil.ReadFile(pc.CertPath)
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
		return nil, errs.New("failed to load identity %#v: %v",
			pc.CertPath, err)
	}

	i, err := NodeIDFromKey(c[0].PublicKey)
	if err != nil {
		return nil, err
	}

	return &PeerCertificateAuthority{
		RestChain: c[1:],
		Cert:      c[0],
		ID:        i,
	}, nil
}

// NewCA creates a new full identity with the given difficulty
func NewCA(ctx context.Context, opts NewCAOptions) (*FullCertificateAuthority, error) {
	if opts.Concurrency < 1 {
		opts.Concurrency = 1
	}
	ctx, cancel := context.WithCancel(ctx)

	eC := make(chan error)
	caC := make(chan FullCertificateAuthority, 1)
	for i := 0; i < int(opts.Concurrency); i++ {
		go newCAWorker(ctx, opts.Difficulty, opts.ParentCert, opts.ParentKey, caC, eC)
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
func (fc FullCAConfig) Save(ca *FullCertificateAuthority) error {
	mode := os.O_WRONLY | os.O_CREATE
	certFile, err := openCert(fc.CertPath, mode)
	if err != nil {
		return err
	}

	keyFile, err := openKey(fc.KeyPath, mode)
	if err != nil {
		return utils.CombineErrors(err, certFile.Close())
	}

	chain := []*x509.Certificate{ca.Cert}
	chain = append(chain, ca.RestChain...)
	certWriteErr := peertls.WriteChain(certFile, chain...)
	keyWriteErr := peertls.WriteKey(keyFile, ca.Key)

	return utils.CombineErrors(certWriteErr, keyWriteErr, certFile.Close(), keyFile.Close())
}

// NewIdentity generates a new `FullIdentity` based on the CA. The CA
// cert is included in the identity's cert chain and the identity's leaf cert
// is signed by the CA.
func (ca FullCertificateAuthority) NewIdentity() (*FullIdentity, error) {
	leafTemplate, err := peertls.LeafTemplate()
	if err != nil {
		return nil, err
	}
	leafKey, err := peertls.NewKey()
	if err != nil {
		return nil, err
	}
	pk, ok := leafKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, peertls.ErrUnsupportedKey.New("%T", leafKey)
	}
	leafCert, err := peertls.NewCert(pk, ca.Key, leafTemplate, ca.Cert)
	if err != nil {
		return nil, err
	}

	if ca.RestChain != nil && len(ca.RestChain) > 0 {
		err := peertls.AddSignedLeafExt(ca.Key, leafCert)
		if err != nil {
			return nil, err
		}
	}

	return &FullIdentity{
		RestChain: ca.RestChain,
		CA:        ca.Cert,
		Leaf:      leafCert,
		Key:       leafKey,
		ID:        ca.ID,
	}, nil
}
