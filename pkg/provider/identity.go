// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"net"

	base58 "github.com/jbenet/go-base58"
	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/peertls"
)

// PeerIdentity represents another peer on the network.
type PeerIdentity struct {
	// CA represents the peer's self-signed CA. The ID is taken from this cert.
	CA *x509.Certificate
	// Leaf represents the leaf they're currently using. The leaf should be
	// signed by the CA. The leaf is what is used for communication.
	Leaf *x509.Certificate
	// The ID is calculated from the CA cert.
	ID dht.NodeID
}

// FullIdentity represents you on the network. In addition to a PeerIdentity,
// a FullIdentity also has a PrivateKey, which a PeerIdentity doesn't have.
// The PrivateKey should be for the PeerIdentity's Leaf certificate.
type FullIdentity struct {
	PeerIdentity
	PrivateKey crypto.PrivateKey

	todoCert *tls.Certificate // TODO(jt): get rid of this and only use the above
}

// IdentityConfig allows you to run a set of Responsibilities with the given
// identity. You can also just load an Identity from disk.
type IdentityConfig struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$CONFDIR/identity.leaf.cert"`
	KeyPath  string `help:"path to the private key for this identity" default:"$CONFDIR/identity.leaf.key"`
	Address  string `help:"address to listen on" default:":7777"`
}

// LoadIdentity loads a FullIdentity from the given configuration
func (ic IdentityConfig) LoadIdentity() (*FullIdentity, error) {
	pi, err := FullIdentityFromFiles(ic.CertPath, ic.KeyPath)
	if err != nil {
		return nil, Error.New("failed to load identity %#v, %#v: %v",
			ic.CertPath, ic.KeyPath, err)
	}
	return pi, nil
}

// Run will run the given responsibilities with the configured identity.
func (ic IdentityConfig) Run(ctx context.Context,
	responsibilities ...Responsibility) (
	err error) {
	defer mon.Task()(&ctx)(&err)

	pi, err := ic.LoadIdentity()
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

	zap.S().Infof("Node %s started", s.Identity().ID)

	return s.Run(ctx)
}

// PeerIdentityFromCertChain loads a PeerIdentity from a chain of certificates
func PeerIdentityFromCertChain(chain [][]byte) (*PeerIdentity, error) {
	// TODO(jt): yeah, this totally does not do the right thing yet
	// TODO(jt): fill this in correctly.
	hash := sha256.Sum256(chain[0]) // TODO(jt): this is wrong
	return &PeerIdentity{
		CA:   nil,                            // TODO(jt)
		Leaf: nil,                            // TODO(jt)
		ID:   nodeID(base58.Encode(hash[:])), // TODO(jt): this is wrong
	}, nil
}

// FullIdentityFromFiles loads a FullIdentity from a certificate chain and
// private key file
func FullIdentityFromFiles(certPath, keyPath string) (*FullIdentity, error) {
	cert, err := peertls.LoadCert(certPath, keyPath)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	peer, err := PeerIdentityFromCertChain(cert.Certificate)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &FullIdentity{
		PeerIdentity: *peer,
		PrivateKey:   cert.PrivateKey,
		todoCert:     cert,
	}, nil
}

type nodeID string

func (n nodeID) String() string { return string(n) }
func (n nodeID) Bytes() []byte  { return []byte(n) }
