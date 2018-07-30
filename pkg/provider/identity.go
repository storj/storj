// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"crypto"
	"crypto/x509"
	"net"
	"encoding/base64"
	"io/ioutil"
	"os"
	"encoding/pem"
	"io"
	"path/filepath"
	"crypto/ecdsa"
	"reflect"

	"golang.org/x/crypto/sha3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/pkg/peertls"
)

const (
	IdentityLength = uint16(256)
)

// PeerIdentity represents another peer on the network.
type PeerIdentity struct {
	// CA represents the peer's self-signed CA. The ID is taken from this cert.
	CA *x509.Certificate
	// Leaf represents the leaf they're currently using. The leaf should be
	// signed by the CA. The leaf is what is used for communication.
	Leaf *x509.Certificate
	// The ID is calculated from the CA cert.
	ID nodeID
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
	CertPath string `help:"path to the certificate chain for this identity" default:"$HOME/.storj/identity.cert"`
	KeyPath  string `help:"path to the private key for this identity" default:"$HOME/.storj/identity.key"`
	Address  string `help:"address to listen on" default:":7777"`
}

// LoadIdentity loads a FullIdentity from the given configuration
func (ic IdentityConfig) LoadIdentity() (*FullIdentity, error) {
	certPEM, err := ioutil.ReadFile(ic.CertPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}

	keyPEM, err := ioutil.ReadFile(ic.KeyPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}

	pi, err := FullIdentityFromPEM(certPEM, keyPEM)
	if err != nil {
		return nil, errs.New("failed to load identity %#v, %#v: %v",
			ic.CertPath, ic.KeyPath, err)
	}
	return pi, nil
}

// SaveIdentity saves a FullIdentity with the given configuration
func (ic IdentityConfig) SaveIdentity(fi *FullIdentity) error {
	if err := os.MkdirAll(filepath.Dir(ic.KeyPath), 600); err != nil {
		return errs.Wrap(err)
	}

	if err := os.MkdirAll(filepath.Dir(ic.CertPath), 600); err != nil {
		return errs.Wrap(err)
	}

	certFile, err := os.OpenFile(ic.CertPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errs.New("unable to open cert file for writing \"%s\"", ic.CertPath, err)
	}

	defer func() {
		if err := certFile.Close(); err != nil {
			zap.S().Error(errs.Wrap(err))
		}
	}()

	keyFile, err := os.OpenFile(ic.KeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errs.New("unable to open key file for writing \"%s\"", ic.KeyPath, err)
	}

	defer func() {
		if err := certFile.Close(); err != nil {
			zap.S().Error(errs.Wrap(err))
		}
	}()

	if err = fi.WriteCertChain(certFile); err != nil {
		return err
	}

	if err = fi.WritePrivateKey(keyFile); err != nil {
		return err
	}

	return nil
}

// WriteCertChain writes the certificate chain (leaf-first) from `FullIdentity` to
// a passed writer, PEM-encoded.
func (fi *FullIdentity) WriteCertChain(w io.Writer) error {
	if err := pem.Encode(w, peertls.NewCertBlock(fi.Leaf.Raw)); err != nil {
		return errs.Wrap(err)
	}

	if err := pem.Encode(w, peertls.NewCertBlock(fi.CA.Raw)); err != nil {
		return errs.Wrap(err)
	}

	return nil
}

// WriteCertChain writes the certificate chain (leaf-first) from `FullIdentity` to
// a passed writer, PEM-encoded.
func (fi *FullIdentity) WritePrivateKey(w io.Writer) error {
	var (
		keyBytes []byte
		err error
	)

	switch privateKey := fi.PrivateKey.(type) {
	case *ecdsa.PrivateKey:
		keyBytes, err = x509.MarshalECPrivateKey(privateKey)
		if err != nil {
			return errs.Wrap(err)
		}
	default:
		return peertls.ErrUnsupportedKey.New("%s", reflect.TypeOf(privateKey))
	}

	if err := pem.Encode(w, peertls.NewKeyBlock(keyBytes)); err != nil {
		return errs.Wrap(err)
	}

	return nil
}

func (ic IdentityConfig) Generate(difficulty uint16, concurrency uint) FullIdentity {
		fiC := make(chan FullIdentity, 1)
		done := make(chan bool, 0)
		for i := 0; i < int(concurrency); i++ {
			go generateCreds(difficulty, fiC, done)
		}

		fi := <-fiC
		close(done)

		return fi
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
	ca, err := x509.ParseCertificate(chain[1])
	if err != nil {
		return nil, errs.Wrap(err)
	}

	leaf, err := x509.ParseCertificate(chain[0])
	if err != nil {
		return nil, errs.Wrap(err)
	}

	caPublicKey, err := x509.MarshalPKIXPublicKey(ca.PublicKey)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	hash := make([]byte, IdentityLength)
	sha3.ShakeSum256(hash, caPublicKey)

	return &PeerIdentity{
		CA:   ca,
		Leaf: leaf,
		ID:   nodeID(base64.URLEncoding.EncodeToString(hash)),
	}, nil
}

// FullIdentityFromPEM loads a FullIdentity from a certificate chain and
// private key file
func FullIdentityFromPEM(chainPEM, keyPEM []byte) (*FullIdentity, error) {
	chainBytes, err := decodePEM(chainPEM)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	keysBytes, err := decodePEM(keyPEM)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// NB: there shouldn't be multiple keys in the key file but if there
	// are, this uses the first one
	privateKey, err := x509.ParseECPrivateKey(keysBytes[0])
	if err != nil {
		return nil, errs.New("unable to parse EC private key", err)
	}
	// cert, err := certFromDERs(chainBytes, keysBytes[0])
	// if err != nil {
	// 	return nil, errs.Wrap(err)
	// }

	pi, err := PeerIdentityFromCertChain(chainBytes)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return &FullIdentity{
		PeerIdentity: *pi,
		PrivateKey:   privateKey,
	}, nil
}

// Difficulty returns the number of trailing zero-value bits in the hash
func (pi *PeerIdentity) Difficulty() uint16 {
	return idDifficulty(pi.ID)
}

// Difficulty returns the number of trailing zero-value bits in the hash
func (fi *FullIdentity) Difficulty() uint16 {
	return idDifficulty(fi.PeerIdentity.ID)
}

// func (fi *FullIdentity) ServerOption(difficulty uint16) grpc.ServerOption {
// 	tlsConfig := &tls.Config{
// 		Certificates: []tls.Certificate{fi.Certificate()},
// 		InsecureSkipVerify: true,
// 		ClientAuth: tls.RequireAnyClientCert,
// 		VerifyPeerCertificate: peertls.VerifyPeerFunc(
// 			peertls.VerifyPeerCertChains,
// 			VerifyPeerIdentityFunc(difficulty),
// 		),
// 	}
//
// 	return grpc.Creds(credentials.NewTLS(tlsConfig))
// }
//
// func (fi *FullIdentity) Certificate() *tls.Certificate {
// 	cert := &tls.Certificate{
// 		Certificate:
// 	}
// }

type nodeID string

func (n nodeID) String() string { return string(n) }
func (n nodeID) Bytes() []byte  { return []byte(n) }
