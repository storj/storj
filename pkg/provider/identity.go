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
	"golang.org/x/crypto/sha3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/utils"
)

const (
	// IdentityLength is the number of bytes required to represent node id
	IdentityLength = uint16(256 / 8) // 256 bits
)

// PeerIdentity represents another peer on the network.
type PeerIdentity struct {
	RestChain []*x509.Certificate
	// CA represents the peer's self-signed CA
	CA *x509.Certificate
	// Leaf represents the leaf they're currently using. The leaf should be
	// signed by the CA. The leaf is what is used for communication.
	Leaf *x509.Certificate
	// The ID taken from the CA public key
	ID storj.NodeID
}

// FullIdentity represents you on the network. In addition to a PeerIdentity,
// a FullIdentity also has a Key, which a PeerIdentity doesn't have.
type FullIdentity struct {
	RestChain []*x509.Certificate
	// CA represents the peer's self-signed CA. The ID is taken from this cert.
	CA *x509.Certificate
	// Leaf represents the leaf they're currently using. The leaf should be
	// signed by the CA. The leaf is what is used for communication.
	Leaf *x509.Certificate
	// The ID taken from the CA public key
	ID storj.NodeID
	// Key is the key this identity uses with the leaf for communication.
	Key crypto.PrivateKey
	// PeerCAWhitelist is a whitelist of CA certs which, if present, restricts which peers this identity will verify as valid;
	// peer certs must be signed by a CA in this list to pass peer certificate verification.
	PeerCAWhitelist []*x509.Certificate
	// VerfyAuthExtSig if true, client leafs which handshake with this identity must contain a valid "authority signature extension"
	// (NB: authority signature extensions are verified against certs in the `PeerCAWhitelist`; i.e. if true, a whitelist must be provided)
	VerifyAuthExtSig bool
}

// IdentitySetupConfig allows you to run a set of Responsibilities with the given
// identity. You can also just load an Identity from disk.
type IdentitySetupConfig struct {
	CertPath  string `help:"path to the certificate chain for this identity" default:"$CONFDIR/identity.cert"`
	KeyPath   string `help:"path to the private key for this identity" default:"$CONFDIR/identity.key"`
	Overwrite bool   `help:"if true, existing identity certs AND keys will overwritten for" default:"false"`
	Version   string `help:"semantic version of identity storage format" default:"0"`
}

// IdentityConfig allows you to run a set of Responsibilities with the given
// identity. You can also just load an Identity from disk.
type IdentityConfig struct {
	CertPath            string `help:"path to the certificate chain for this identity" default:"$CONFDIR/identity.cert"`
	KeyPath             string `help:"path to the private key for this identity" default:"$CONFDIR/identity.key"`
	PeerCAWhitelistPath string `help:"path to the CA cert whitelist (peer identities must be signed by one these to be verified)"`
	VerifyAuthExtSig    bool   `help:"if true, client leafs must contain a valid \"authority signature extension\" (NB: authority signature extensions are verified against certs in the peer ca whitelist; i.e. if true, a whitelist must be provided)" default:"false"`
	Address             string `help:"address to listen on" default:":7777"`
}

// FullIdentityFromPEM loads a FullIdentity from a certificate chain and
// private key file
func FullIdentityFromPEM(chainPEM, keyPEM, CAWhitelistPEM []byte) (*FullIdentity, error) {
	cb, err := decodePEM(chainPEM)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	if len(cb) < 2 {
		return nil, errs.New("too few certificates in chain")
	}
	kb, err := decodePEM(keyPEM)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	// NB: there shouldn't be multiple keys in the key file but if there
	// are, this uses the first one
	k, err := x509.ParseECPrivateKey(kb[0])
	if err != nil {
		return nil, errs.New("unable to parse EC private key: %v", err)
	}
	ch, err := ParseCertChain(cb)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	i, err := NodeIDFromKey(ch[1].PublicKey)
	if err != nil {
		return nil, err
	}

	var (
		wb        [][]byte
		whitelist []*x509.Certificate
	)
	if CAWhitelistPEM != nil {
		wb, err = decodePEM(CAWhitelistPEM)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		whitelist, err = ParseCertChain(wb)
		if err != nil {
			return nil, errs.Wrap(err)
		}
	}

	return &FullIdentity{
		RestChain:       ch[2:],
		CA:              ch[1],
		Leaf:            ch[0],
		Key:             k,
		ID:              i,
		PeerCAWhitelist: whitelist,
	}, nil
}

// ParseCertChain converts a chain of certificate bytes into x509 certs
func ParseCertChain(chain [][]byte) ([]*x509.Certificate, error) {
	c := make([]*x509.Certificate, len(chain))
	for i, ct := range chain {
		cp, err := x509.ParseCertificate(ct)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		c[i] = cp
	}
	return c, nil
}

// PeerIdentityFromCerts loads a PeerIdentity from a pair of leaf and ca x509 certificates
func PeerIdentityFromCerts(leaf, ca *x509.Certificate, rest []*x509.Certificate) (*PeerIdentity, error) {
	i, err := NodeIDFromKey(ca.PublicKey.(crypto.PublicKey))
	if err != nil {
		return nil, err
	}

	return &PeerIdentity{
		RestChain: rest,
		CA:        ca,
		ID:        i,
		Leaf:      leaf,
	}, nil
}

// PeerIdentityFromPeer loads a PeerIdentity from a peer connection
func PeerIdentityFromPeer(peer *peer.Peer) (*PeerIdentity, error) {
	tlsInfo := peer.AuthInfo.(credentials.TLSInfo)
	c := tlsInfo.State.PeerCertificates
	if len(c) < 2 {
		return nil, Error.New("invalid certificate chain")
	}
	pi, err := PeerIdentityFromCerts(c[0], c[1], c[2:])
	if err != nil {
		return nil, err
	}

	return pi, nil
}

// PeerIdentityFromContext loads a PeerIdentity from a ctx TLS credentials
func PeerIdentityFromContext(ctx context.Context) (*PeerIdentity, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, Error.New("unable to get grpc peer from contex")
	}

	return PeerIdentityFromPeer(p)
}

// Stat returns the status of the identity cert/key files for the config
func (is IdentitySetupConfig) Stat() TLSFilesStatus {
	return statTLSFiles(is.CertPath, is.KeyPath)
}

// Create generates and saves a CA using the config
func (is IdentitySetupConfig) Create(ca *FullCertificateAuthority) (*FullIdentity, error) {
	fi, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	fi.CA = ca.Cert
	ic := IdentityConfig{
		CertPath: is.CertPath,
		KeyPath:  is.KeyPath,
	}
	return fi, ic.Save(fi)
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
	w, err := ioutil.ReadFile(ic.PeerCAWhitelistPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	fi, err := FullIdentityFromPEM(c, k, w)
	if err != nil {
		return nil, errs.New("failed to load identity %#v, %#v: %v",
			ic.CertPath, ic.KeyPath, err)
	}
	return fi, nil
}

// Save saves a FullIdentity according to the config
func (ic IdentityConfig) Save(fi *FullIdentity) error {
	f := os.O_WRONLY | os.O_CREATE
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

	chain := []*x509.Certificate{fi.Leaf, fi.CA}
	chain = append(chain, fi.RestChain...)
	if err = peertls.WriteChain(c, chain...); err != nil {
		return err
	}
	if err = peertls.WriteKey(k, fi.Key); err != nil {
		return err
	}
	return nil
}

// Run will run the given responsibilities with the configured identity.
func (ic IdentityConfig) Run(ctx context.Context, interceptor grpc.UnaryServerInterceptor, responsibilities ...Responsibility) (err error) {
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

	s, err := NewProvider(pi, lis, interceptor, responsibilities...)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()
	zap.S().Infof("Node %s started", s.Identity().ID)
	return s.Run(ctx)
}

// RestChainRaw returns the rest (excluding leaf and CA) of the certficate chain as a 2d byte slice
func (fi *FullIdentity) RestChainRaw() [][]byte {
	var chain [][]byte
	for _, cert := range fi.RestChain {
		chain = append(chain, cert.Raw)
	}
	return chain
}

// ServerOption returns a grpc `ServerOption` for incoming connections
// to the node with this full identity
func (fi *FullIdentity) ServerOption(pcvFuncs ...peertls.PeerCertVerificationFunc) (grpc.ServerOption, error) {
	ch := [][]byte{fi.Leaf.Raw, fi.CA.Raw}
	ch = append(ch, fi.RestChainRaw()...)
	c, err := peertls.TLSCert(ch, fi.Leaf, fi.Key)
	if err != nil {
		return nil, err
	}

	pcvFuncs = append(
		[]peertls.PeerCertVerificationFunc{peertls.VerifyPeerCertChains},
		pcvFuncs...,
	)
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{*c},
		InsecureSkipVerify: true,
		ClientAuth:         tls.RequireAnyClientCert,
		VerifyPeerCertificate: peertls.VerifyPeerFunc(
			pcvFuncs...,
		),
	}

	return grpc.Creds(credentials.NewTLS(tlsConfig)), nil
}

// DialOption returns a grpc `DialOption` for making outgoing connections
// to the node with this peer identity
// id is an optional id of the node we are dialing
func (fi *FullIdentity) DialOption(id storj.NodeID) (grpc.DialOption, error) {
	// TODO(coyle): add ID
	ch := [][]byte{fi.Leaf.Raw, fi.CA.Raw}
	ch = append(ch, fi.RestChainRaw()...)
	c, err := peertls.TLSCert(ch, fi.Leaf, fi.Key)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{*c},
		InsecureSkipVerify: true,
		VerifyPeerCertificate: peertls.VerifyPeerFunc(
			peertls.VerifyPeerCertChains,
			func(_ [][]byte, parsedChains [][]*x509.Certificate) error {
				if id == (storj.NodeID{}) {
					return nil
				}

				peer, err := PeerIdentityFromCerts(parsedChains[0][0], parsedChains[0][1], parsedChains[0][2:])
				if err != nil {
					return err
				}

				if peer.ID.String() != id.String() {
					return Error.New("peer ID did not match requested ID")
				}

				return nil
			},
		),
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}

func NodeIDFromKey(k crypto.PublicKey) (storj.NodeID, error) {
	kb, err := x509.MarshalPKIXPublicKey(k)
	if err != nil {
		return storj.NodeID{}, storj.ErrNodeID.Wrap(err)
	}
	hash := make([]byte, len(storj.NodeID{}))
	sha3.ShakeSum256(hash, kb)
	return storj.NodeIDFromBytes(hash)
}

// NewFullIdentity creates a new ID for nodes with difficulty and concurrency params
func NewFullIdentity(ctx context.Context, difficulty uint16, concurrency uint) (*FullIdentity, error) {
	ca, err := NewCA(ctx, NewCAOptions{
		Difficulty:  difficulty,
		Concurrency: concurrency,
	})
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	return identity, err
}
