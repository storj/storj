// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"os"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"encoding/base64"
	"fmt"
	"math/bits"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/utils"
)

const (
	IdentityLength = uint16(256)
)

var (
	ErrDifficulty = errs.Class("difficulty error")
)

// PeerIdentity represents another peer on the network.
type PeerIdentity struct {
	// CA represents the peer's self-signed CA. The ID is taken from this cert.
	CA CertificateAuthority
	// Leaf represents the leaf they're currently using. The leaf should be
	// signed by the CA. The leaf is what is used for communication.
	Leaf *x509.Certificate
}

// FullIdentity represents you on the network. In addition to a PeerIdentity,
// a FullIdentity also has a PrivateKey, which a PeerIdentity doesn't have.
// The PrivateKey should be for the PeerIdentity's Leaf certificate.
type FullIdentity struct {
	PeerIdentity
	PrivateKey crypto.PrivateKey
}

// IdentityConfig allows you to run a set of Responsibilities with the given
// identity. You can also just load an Identity from disk.
type IdentitySetupConfig struct {
	IdentityConfig
	Overwrite bool `help:"if true, existing identity certs AND keys will overwritten for" default:"false"`
}

// IdentityConfig allows you to run a set of Responsibilities with the given
// identity. You can also just load an Identity from disk.
type IdentityConfig struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$CONFDIR/identity.cert"`
	KeyPath  string `help:"path to the private key for this identity" default:"$CONFDIR/identity.key"`
	Version  string `help:"semantic version of identity storage format" default:"0"`
	Address  string `help:"address to listen on" default:":7777"`
}

// FullIdentityFromPEM loads a FullIdentity from a certificate chain and
// private key file
func FullIdentityFromPEM(chainPEM, keyPEM []byte) (*FullIdentity, error) {
	cb, err := decodePEM(chainPEM)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	kb, err := decodePEM(keyPEM)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// NB: there shouldn't be multiple keys in the key file but if there
	// are, this uses the first one
	pk, err := x509.ParseECPrivateKey(kb[0])
	if err != nil {
		return nil, errs.New("unable to parse EC private key", err)
	}

	pi, err := PeerIdentityFromCertChain(cb)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return &FullIdentity{
		PeerIdentity: *pi,
		PrivateKey:   pk,
	}, nil
}

// PeerIdentityFromCertChain loads a PeerIdentity from a chain of certificates
func PeerIdentityFromCertChain(chain [][]byte) (*PeerIdentity, error) {
	var (
		cb []byte
		lb []byte
	)
	switch len(chain) {
	case 0:
		return nil, errs.New("empty certificate chain")
	case 1:
		cb = chain[0]
	default:
		cb = chain[1]
	}
	lb = chain[0]

	ca, err := x509.ParseCertificate(cb)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	l, err := x509.ParseCertificate(lb)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return PeerIdentityFromCerts(l, ca)
}

// PeerIdentityFromCerts loads a PeerIdentity from a pair of leaf and ca x509 certificates
func PeerIdentityFromCerts(leaf, ca *x509.Certificate) (*PeerIdentity, error) {
	i, err := idFromKey(ca.PublicKey.(crypto.PublicKey))
	if err != nil {
		return nil, err
	}

	return &PeerIdentity{
		CA: CertificateAuthority{
			Cert: ca,
			ID:   i,
		},
		Leaf: leaf,
	}, nil
}

// VerifyPeerIdentityFunc returns a function to use with `tls.Certificate.VerifyPeerCertificate`
// that verifies the peer identity satisfies the minimum difficulty
func VerifyPeerIdentityFunc(difficulty uint16) peertls.PeerCertVerificationFunc {
	return func(rawChain [][]byte, parsedChains [][]*x509.Certificate) error {
		// NB: use the first chain; leaf should be first, followed by the ca
		pi, err := PeerIdentityFromCerts(parsedChains[0][0], parsedChains[0][1])
		if err != nil {
			return err
		}

		if pi.Difficulty() < difficulty {
			return ErrDifficulty.New("expected: \"%d\" but got: \"%d\"", difficulty, pi.Difficulty())
		}

		return nil
	}
}

// LoadOrCreate loads or generates the identity files using the configuration
func (ic IdentitySetupConfig) LoadOrCreate(ca *CertificateAuthority) (*FullIdentity, error) {
	var (
		fi  = new(FullIdentity)
		err error
	)
	switch ic.Stat() {
	case NoCertKey:
		if ic.Overwrite {
			zap.S().Warn("overwriting identity")
			fi, err = ic.Create(ca)
			if err != nil {
				return nil, err
			}
			break
		}

		return nil, errs.New("a key already exists for identity at \"%s\" " +
			"but no cert was found at \"%s\"; if you wish overwrite this key, set " +
			"the overwrite option to true")
	case CertKey | CertNoKey:
		if ic.Overwrite {
			zap.S().Warn("overwriting identity")
			fi, err = ic.Create(ca)
			if err != nil {
				return nil, err
			}
			break
		}

		zap.S().Info("identity exist, loading")
		fi, err = ic.Load()
		if err != nil {
			return nil, err
		}
	case NoCertNoKey:
		zap.S().Info("identity not found, generating")
		fi, err = ic.Create(ca)
		if err != nil {
			return nil, err
		}
	}
	return fi, nil
}

// Load loads a FullIdentity from the config
func (ic IdentityConfig) Load() (*FullIdentity, error) {
	c, err := ioutil.ReadFile(ic.CertPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}
	k, err := ioutil.ReadFile(ic.KeyPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}

	fi, err := FullIdentityFromPEM(c, k)
	if err != nil {
		return nil, errs.New("failed to load identity %#v, %#v: %v",
			ic.CertPath, ic.KeyPath, err)
	}
	return fi, nil
}

// Create generates and saves a CA using the config
func (ic IdentityConfig) Create(ca *CertificateAuthority) (*FullIdentity, error) {
	fi, err := ca.GenerateIdentity()
	if err != nil {
		return nil, err
	}
	return fi, ic.Save(fi)
}

// Save saves a FullIdentity according to the config
func (ic IdentityConfig) Save(fi *FullIdentity) error {
	f := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	c, err := openCert(ic.CertPath, f)
	if err != nil {
		return err
	}
	defer utils.LogClose(c)
	k, err := openKey(ic.KeyPath, f)
	if err != nil {
		return err
	}
	defer utils.LogClose(k)

	if err = peertls.WriteChain(c, fi.Leaf, fi.CA.Cert); err != nil {
		return err
	}
	if err = peertls.WriteKey(k, fi.PrivateKey); err != nil {
		return err
	}
	return nil
}

// Stat returns the status of the identity cert/key files for the config
func (ic IdentityConfig) Stat() TlsFilesStat {
	return statTLSFiles(ic.CertPath, ic.KeyPath)
}

// Run will run the given responsibilities with the configured identity.
func (ic IdentityConfig) Run(ctx context.Context,
	responsibilities ...Responsibility) (
	err error) {
	defer mon.Task()(&ctx)(&err)

	pi, err := ic.Load()
	if err != nil {
		return err
	}

	lis, err := net.Listen("tcp", ic.Address)
	if err != nil {
		return err
	}
	defer func() { _ = lis.Close() }()

	s, err := NewProvider(pi, lis, responsibilities...)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	zap.S().Infof("Node %s started", s.Identity().ID())

	return s.Run(ctx)
}

// ID returns the ID of the certificate authority associated with this `FullIdentity`
func (fi *FullIdentity) ID() nodeID {
	return fi.CA.ID
}

// ID returns the ID of the certificate authority associated with this `PeerIdentity`
func (pi *PeerIdentity) ID() nodeID {
	return pi.CA.ID
}

// Difficulty returns the number of trailing zero-value bits in the CA's ID hash
func (fi *FullIdentity) Difficulty() uint16 {
	return fi.ID().Difficulty()
}

// Difficulty returns the number of trailing zero-value bits in the CA's ID hash
func (pi *PeerIdentity) Difficulty() uint16 {
	return pi.ID().Difficulty()
}

// ServerOption returns a grpc `ServerOption` for incoming connections
// to the node with this full identity
func (fi *FullIdentity) ServerOption(difficulty uint16) (grpc.ServerOption, error) {
	c, err := fi.Certificate()
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{*c},
		InsecureSkipVerify: true,
		ClientAuth:         tls.RequireAnyClientCert,
		VerifyPeerCertificate: peertls.VerifyPeerFunc(
			peertls.VerifyPeerCertChains,
			VerifyPeerIdentityFunc(difficulty),
		),
	}

	return grpc.Creds(credentials.NewTLS(tlsConfig)), nil
}

// DialOption returns a grpc `DialOption` for making outgoing connections
// to the node with this peer identity
func (pi *PeerIdentity) DialOption(difficulty uint16) (grpc.DialOption, error) {
	c, err := pi.Certificate()
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{*c},
		InsecureSkipVerify: true,
		VerifyPeerCertificate: peertls.VerifyPeerFunc(
			peertls.VerifyPeerCertChains,
			VerifyPeerIdentityFunc(difficulty),
		),
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}

// Certificate returns a `*tls.Certifcate` using the identity's certificate and key
func (fi *FullIdentity) Certificate() (*tls.Certificate, error) {
	var chain [][]byte
	chain = append(chain, fi.Leaf.Raw, fi.CA.Cert.Raw)

	return peertls.TLSCert(chain, fi.Leaf, fi.PrivateKey)
}

// Certificate returns a `*tls.Certifcate` using the identity's certificate and key
func (pi *PeerIdentity) Certificate() (*tls.Certificate, error) {
	var chain [][]byte
	chain = append(chain, pi.Leaf.Raw, pi.CA.Cert.Raw)

	return peertls.TLSCert(chain, pi.Leaf, nil)
}

type nodeID string

func (n nodeID) String() string { return string(n) }
func (n nodeID) Bytes() []byte  { return []byte(n) }
func (n nodeID) Difficulty() uint16 {
	hash, err := base64.URLEncoding.DecodeString(n.String())
	if err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	for i := 1; i < len(hash); i++ {
		b := hash[len(hash)-i]

		if b != 0 {
			zeroBits := bits.TrailingZeros16(uint16(b))
			if zeroBits == 16 {
				zeroBits = 0
			}

			return uint16((i-1)*8 + zeroBits)
		}
	}

	// NB: this should never happen
	reason := fmt.Sprintf("difficulty matches hash length! hash: %s", hash)
	zap.S().Error(reason)
	panic(reason)
}
