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

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/utils"
)

const (
	IdentityLength = uint16(256)
)

var (
	ErrDifficulty = errs.Class("difficulty error")
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

type CAConfig struct {
	CertPath   string `help:"path to the certificate chain for this identity" default:"$CONFDIR/identity.leaf.cert"`
	KeyPath    string `help:"path to the private key for this identity" default:"$CONFDIR/identity.leaf.key"`
	Difficulty uint16 `default:"24" help:"minimum difficulty for identity generation"`
	Version    string `help:"semantic version of CA storage format"`
}

// IdentityConfig allows you to run a set of Responsibilities with the given
// identity. You can also just load an Identity from disk.
type IdentityConfig struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$CONFDIR/identity.leaf.cert"`
	KeyPath  string `help:"path to the private key for this identity" default:"$CONFDIR/identity.leaf.key"`
	Version  string `help:"semantic version of identity storage format"`
	Address  string `help:"address to listen on" default:":7777"`
}

// GenerateCA creates a new full identity with the given difficulty
func GenerateCA(ctx context.Context, difficulty uint16, concurrency uint) *CertificateAuthority {
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
	ca, err := x509.ParseCertificate(chain[1])
	if err != nil {
		return nil, errs.Wrap(err)
	}

	l, err := x509.ParseCertificate(chain[0])
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return PeerIdentityFromCerts(l, ca)
}

// PeerIdentityFromCerts loads a PeerIdentity from a pair of leaf and ca x509 certificates
func PeerIdentityFromCerts(leaf, ca *x509.Certificate) (*PeerIdentity, error) {
	i, err := idFromCert(ca)
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

// Generate Identity
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

	pi, err := PeerIdentityFromCertChain([][]byte{cb})
	if err != nil {
		return nil, errs.New("failed to load identity %#v, %#v: %v",
			caC.CertPath, caC.KeyPath, err)
	}

	k, err := x509.ParseECPrivateKey(kb)
	if err != nil {
		return nil, errs.New("unable to parse EC private key", err)
	}
	pi.CA.PrivateKey = k

	return &pi.CA, nil
}

// Load loads a FullIdentity from the given configuration
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

// Save saves a FullIdentity with the given configuration
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

func (ca *CertificateAuthority) Difficulty() uint16 {
	return idDifficulty(ca.ID)
}

// Difficulty returns the number of trailing zero-value bits in the CA's ID hash
func (fi *FullIdentity) Difficulty() uint16 {
	return idDifficulty(fi.ID())
}

// Difficulty returns the number of trailing zero-value bits in the CA's ID hash
func (pi *PeerIdentity) Difficulty() uint16 {
	return idDifficulty(pi.ID())
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

// Certificate converts the full identity `*tls.Certificate` into a
// `*tlsCertificate`
func (fi *FullIdentity) Certificate() (*tls.Certificate, error) {
	var chain [][]byte
	chain = append(chain, fi.Leaf.Raw, fi.CA.Cert.Raw)

	return peertls.TLSCert(chain, fi.Leaf, fi.PrivateKey)
}

// Certificate converts the peer identity `*tls.Certificate` into a
// `*tlsCertificate` (without a private key)
func (pi *PeerIdentity) Certificate() (*tls.Certificate, error) {
	var chain [][]byte
	chain = append(chain, pi.Leaf.Raw, pi.CA.Cert.Raw)

	return peertls.TLSCert(chain, pi.Leaf, nil)
}

type nodeID string

func (n nodeID) String() string { return string(n) }
func (n nodeID) Bytes() []byte  { return []byte(n) }
