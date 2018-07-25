// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"crypto"
	"crypto/x509"
	"net"
	"encoding/base64"
	"path/filepath"
	"os"
	"io/ioutil"

	"golang.org/x/crypto/sha3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/peertls"
	"crypto/tls"
)

const (
	DefaultHashLength = uint16(256)
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
}

// IdentityConfig allows you to run a set of Responsibilities with the given
// identity. You can also just load an Identity from disk.
type IdentityConfig struct {
	Path    string `help:"path to the dentity file (PEM-encoded certificate chain & leaf private key)" default:"$HOME/.storj/identity.pem"`
	Address string `help:"address to listen on" default:":7777"`
}

// LoadIdentity loads a FullIdentity from the given configuration
func (ic IdentityConfig) LoadIdentity() (*FullIdentity, error) {
	id, err := FullIdentityFromFile(ic.Path)
	if err != nil {
		return nil, Error.New("failed to load identity %#v, %#v: %v", ic.Path, err)
	}
	return id, nil
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
func PeerIdentityFromCertChain(cert *tls.Certificate) (*PeerIdentity, error) {
	ca, err := x509.ParseCertificate(cert.Certificate[len(cert.Certificate)])
	if err != nil {
		return nil, errs.Wrap(err)
	}

	hash := make([]byte, DefaultHashLength)
	sha3.ShakeSum256(hash, ca.RawTBSCertificate)

	return &PeerIdentity{
		CA:   ca,
		Leaf: cert.Leaf,
		ID:   nodeID(base64.URLEncoding.EncodeToString(hash)),
	}, nil
}

// FullIdentityFromFile loads a FullIdentity from a certificate chain and
// private key file
func FullIdentityFromFile(path string) (*FullIdentity, error) {
	baseDir := filepath.Dir(path)

	if _, err := os.Stat(baseDir); err != nil {
		if err == os.ErrNotExist {
			return nil, peertls.ErrNotExist.Wrap(err)
		}

		return nil, errs.Wrap(err)
	}

	IDBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}

	cert, err := parseIDBytes(IDBytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	peer, err := PeerIdentityFromCertChain(cert)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &FullIdentity{
		PeerIdentity: *peer,
		PrivateKey:   cert.PrivateKey,
	}, nil
}

type nodeID string

func (n nodeID) String() string { return string(n) }
func (n nodeID) Bytes() []byte  { return []byte(n) }
