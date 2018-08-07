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
	"go.uber.org/zap"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/utils"
)

// CertificateAuthority represents the CA which is used to author and validate identities
type CertificateAuthority struct {
	// PrivateKey is the private key of the CA
	PrivateKey crypto.PrivateKey
	// Cert is the x509 certificate of the CA
	Cert *x509.Certificate
	// The ID is calculated from the CA cert.
	ID nodeID
}

type CASetupConfig struct {
	CAConfig
	Timeout     string `help:"timeout for CA generation; golang duration string (0 no timeout)" default:"5m"`
	Overwrite   bool   `help:"if true, existing CA certs AND keys will overwritten" default:"false"`
	Concurrency uint   `help:"number of concurrent workers for certificate authority generation" default:"4"`
}

type CAConfig struct {
	CertPath   string `help:"path to the certificate chain for this identity" default:"$CONFDIR/ca.cert"`
	KeyPath    string `help:"path to the private key for this identity" default:"$CONFDIR/ca.key"`
	Difficulty uint64 `help:"minimum difficulty for identity generation" default:"24"`
	Version    string `help:"semantic version of CA storage format" default:"0"`
}

// LoadOrCreate loads or generates the CA files using the configuration
func (caC CASetupConfig) LoadOrCreate(ctx context.Context, concurrency uint) (*CertificateAuthority, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var ca = new(CertificateAuthority)
	load := func() (*CertificateAuthority, error) {
		ca, err := caC.Load()
		if err != nil {
			return nil, err
		}

		if ca.Difficulty() < uint16(caC.Difficulty) {
			return nil, ErrDifficulty.New("loaded certificate authority has a difficulty less than requested: %d; expected >= %d",
				ca.Difficulty(), caC.Difficulty)
		}

		return ca, nil
	}

	var (
		new bool
		err error
	)
	switch caC.Stat() {
	case NoCertKey:
		if caC.Overwrite {
			zap.S().Warn("overwriting certificate authority")
			new = true
			ca, err = caC.Create(ctx, concurrency)
			if err != nil {
				return nil, new, err
			}
			break
		}

		new = false
		return nil, new, errs.New("a key already exists at \"%s\" but no cert was found at \"%s\"; " +
			"if you wish overwrite this key, set the overwrite option to true")
	case CertKey | CertNoKey:
		if caC.Overwrite {
			zap.S().Info("overwriting certificate authority")
			new = true
			ca, err = caC.Create(ctx, concurrency)
			if err != nil {
				return nil, new, err
			}
			break
		}

		zap.S().Info("certificate authority exist, loading")
		new = false
		ca, err = load()
		if err != nil {
			return nil, false, err
		}
	case NoCertNoKey:
		zap.S().Info("certificate authority not found, generating")
		new = true
		ca, err = caC.Create(ctx, concurrency)
		if err != nil {
			return nil, new, err
		}
	}
	return ca, new, nil
}

// Load loads a CA from the given configuration
func (caC CAConfig) Load() (*CertificateAuthority, error) {
	cb, err := ioutil.ReadFile(caC.CertPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}
	kb, err := ioutil.ReadFile(caC.KeyPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}

	var c [][]byte
	for {
		var cp *pem.Block
		cp, cb = pem.Decode(cb)
		if cp == nil {
			break
		}
		c = append(c, cp.Bytes)
	}
	pi, err := PeerIdentityFromCertChain(c)
	if err != nil {
		return nil, errs.New("failed to load identity %#v, %#v: %v",
			caC.CertPath, caC.KeyPath, err)
	}

	kp, _ := pem.Decode(kb)
	k, err := x509.ParseECPrivateKey(kp.Bytes)
	if err != nil {
		return nil, errs.New("unable to parse EC private key", err)
	}
	pi.CA.PrivateKey = k

	return &pi.CA, nil
}

// Create generates and saves a CA using the config
func (caC CAConfig) Create(ctx context.Context, concurrency uint) (*CertificateAuthority, error) {
	ca := GenerateCA(ctx, uint16(caC.Difficulty), concurrency)
	return ca, caC.Save(ca)
}

// GenerateCA creates a new full identity with the given difficulty
func GenerateCA(ctx context.Context, difficulty uint16, concurrency uint) *CertificateAuthority {
	if concurrency < 1 {
		concurrency = 1
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)

	caC := make(chan CertificateAuthority, 1)
	for i := 0; i < int(concurrency); i++ {
		go generateCAWorker(ctx, difficulty, caC)
	}

	ca := <-caC
	cancel()

	return &ca
}

// Save saves a CA with the given configuration
func (caC CAConfig) Save(ca *CertificateAuthority) error {
	f := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
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
	if err = peertls.WriteKey(k, ca.PrivateKey); err != nil {
		return err
	}
	return nil
}

// Stat returns the status of the CA cert/key files for the config
func (caC CAConfig) Stat() TlsFilesStat {
	return statTLSFiles(caC.CertPath, caC.KeyPath)
}

// Generate Identity generates a new `FullIdentity` based on the CA. The CA
// cert is included in the identity's cert chain and the identity's leaf cert
// is signed by the CA.
func (ca CertificateAuthority) GenerateIdentity() (*FullIdentity, error) {
	lT, err := peertls.LeafTemplate()
	if err != nil {
		return nil, err
	}
	caC, err := peertls.TLSCert([][]byte{ca.Cert.Raw}, ca.Cert, ca.PrivateKey)
	if err != nil {
		return nil, err
	}
	lC, err := peertls.Generate(lT, ca.Cert, caC, caC)
	if err != nil {
		return nil, err
	}
	pi, err := PeerIdentityFromCerts(lC.Leaf, ca.Cert)
	if err != nil {
		return nil, err
	}

	return &FullIdentity{
		PeerIdentity: *pi,
		PrivateKey:   lC.PrivateKey,
	}, nil
}
