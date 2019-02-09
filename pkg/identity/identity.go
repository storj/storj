// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
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
}

// SetupConfig allows you to run a set of Responsibilities with the given
// identity. You can also just load an Identity from disk.
type SetupConfig struct {
	CertPath  string `help:"path to the certificate chain for this identity" default:"$IDENTITYDIR/identity.cert"`
	KeyPath   string `help:"path to the private key for this identity" default:"$IDENTITYDIR/identity.key"`
	Overwrite bool   `help:"if true, existing identity certs AND keys will overwritten for" default:"false"`
	Version   string `help:"semantic version of identity storage format" default:"0"`
}

// Config allows you to run a set of Responsibilities with the given
// identity. You can also just load an Identity from disk.
type Config struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$IDENTITYDIR/identity.cert" user:"true"`
	KeyPath  string `help:"path to the private key for this identity" default:"$IDENTITYDIR/identity.key" user:"true"`
}

// PeerConfig allows you to interact with a peer identity (cert, no key) on disk.
type PeerConfig struct {
	CertPath string `help:"path to the certificate chain for this identity" default:"$IDENTITYDIR/identity.cert" user:"true"`
}

// FullIdentityFromPEM loads a FullIdentity from a certificate chain and
// private key PEM-encoded bytes
func FullIdentityFromPEM(chainPEM, keyPEM []byte) (*FullIdentity, error) {
	peerIdent, err := PeerIdentityFromPEM(chainPEM)
	if err != nil {
		return nil, err
	}

	// NB: there shouldn't be multiple keys in the key file but if there
	// are, this uses the first one
	key, err := pkcrypto.PrivateKeyFromPEM(keyPEM)
	if err != nil {
		return nil, err
	}

	return &FullIdentity{
		RestChain: peerIdent.RestChain,
		CA:        peerIdent.CA,
		Leaf:      peerIdent.Leaf,
		Key:       key,
		ID:        peerIdent.ID,
	}, nil
}

// PeerIdentityFromPEM loads a PeerIdentity from a certificate chain and
// private key PEM-encoded bytes
func PeerIdentityFromPEM(chainPEM []byte) (*PeerIdentity, error) {
	chain, err := pkcrypto.CertsFromPEM(chainPEM)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	if len(chain) < peertls.CAIndex+1 {
		return nil, pkcrypto.ErrChainLength.New("identity chain does not contain a CA certificate")
	}
	nodeID, err := NodeIDFromKey(chain[peertls.CAIndex].PublicKey)
	if err != nil {
		return nil, err
	}

	return &PeerIdentity{
		RestChain: chain[peertls.CAIndex+1:],
		CA:        chain[peertls.CAIndex],
		Leaf:      chain[peertls.LeafIndex],
		ID:        nodeID,
	}, nil
}

// PeerIdentityFromCerts loads a PeerIdentity from a pair of leaf and ca x509 certificates
func PeerIdentityFromCerts(leaf, ca *x509.Certificate, rest []*x509.Certificate) (*PeerIdentity, error) {
	i, err := NodeIDFromKey(ca.PublicKey)
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
	pi, err := PeerIdentityFromCerts(c[peertls.LeafIndex], c[peertls.CAIndex], c[2:])
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

// NodeIDFromCertPath loads a node ID from a certificate file path
func NodeIDFromCertPath(certPath string) (storj.NodeID, error) {
	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		return storj.NodeID{}, err
	}
	return NodeIDFromPEM(certBytes)
}

// NodeIDFromPEM loads a node ID from certificate bytes
func NodeIDFromPEM(pemBytes []byte) (storj.NodeID, error) {
	chain, err := pkcrypto.CertsFromPEM(pemBytes)
	if err != nil {
		return storj.NodeID{}, Error.New("invalid identity certificate")
	}
	if len(chain) < peertls.CAIndex+1 {
		return storj.NodeID{}, Error.New("no CA in identity certificate")
	}
	return NodeIDFromKey(chain[peertls.CAIndex].PublicKey)
}

// NodeIDFromKey hashes a public key and creates a node ID from it
func NodeIDFromKey(k crypto.PublicKey) (storj.NodeID, error) {
	// id = sha256(sha256(pkix(k)))
	kb, err := x509.MarshalPKIXPublicKey(k)
	if err != nil {
		return storj.NodeID{}, storj.ErrNodeID.Wrap(err)
	}
	mid := sha256.Sum256(kb)
	end := sha256.Sum256(mid[:])
	return storj.NodeID(end), nil
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

// Status returns the status of the identity cert/key files for the config
func (is SetupConfig) Status() TLSFilesStatus {
	return statTLSFiles(is.CertPath, is.KeyPath)
}

// Create generates and saves a CA using the config
func (is SetupConfig) Create(ca *FullCertificateAuthority) (*FullIdentity, error) {
	fi, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	fi.CA = ca.Cert
	ic := Config{
		CertPath: is.CertPath,
		KeyPath:  is.KeyPath,
	}
	return fi, ic.Save(fi)
}

// FullConfig converts a `SetupConfig` to `Config`
func (is SetupConfig) FullConfig() Config {
	return Config{
		CertPath: is.CertPath,
		KeyPath:  is.KeyPath,
	}
}

// Load loads a FullIdentity from the config
func (ic Config) Load() (*FullIdentity, error) {
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

// Save saves a FullIdentity according to the config
func (ic Config) Save(fi *FullIdentity) error {
	var (
		certData, keyData                                              bytes.Buffer
		writeChainErr, writeChainDataErr, writeKeyErr, writeKeyDataErr error
	)

	chain := []*x509.Certificate{fi.Leaf, fi.CA}
	chain = append(chain, fi.RestChain...)

	if ic.CertPath != "" {
		writeChainErr = peertls.WriteChain(&certData, chain...)
		writeChainDataErr = writeChainData(ic.CertPath, certData.Bytes())
	}

	if ic.KeyPath != "" {
		writeKeyErr = pkcrypto.WritePrivateKeyPEM(&keyData, fi.Key)
		writeKeyDataErr = writeKeyData(ic.KeyPath, keyData.Bytes())
	}

	writeErr := utils.CombineErrors(writeChainErr, writeKeyErr)
	if writeErr != nil {
		return writeErr
	}

	return utils.CombineErrors(
		writeChainDataErr,
		writeKeyDataErr,
	)
}

// SaveBackup saves the certificate of the config with a timestamped filename
func (ic Config) SaveBackup(fi *FullIdentity) error {
	return Config{
		CertPath: backupPath(ic.CertPath),
		KeyPath:  backupPath(ic.KeyPath),
	}.Save(fi)
}

// PeerConfig converts a Config to a PeerConfig
func (ic Config) PeerConfig() *PeerConfig {
	return &PeerConfig{
		CertPath: ic.CertPath,
	}
}

// Load loads a PeerIdentity from the config
func (ic PeerConfig) Load() (*PeerIdentity, error) {
	c, err := ioutil.ReadFile(ic.CertPath)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}
	pi, err := PeerIdentityFromPEM(c)
	if err != nil {
		return nil, errs.New("failed to load identity %#v: %v",
			ic.CertPath, err)
	}
	return pi, nil
}

// Save saves a PeerIdentity according to the config
func (ic PeerConfig) Save(fi *PeerIdentity) error {
	var (
		certData                         bytes.Buffer
		writeChainErr, writeChainDataErr error
	)

	chain := []*x509.Certificate{fi.Leaf, fi.CA}
	chain = append(chain, fi.RestChain...)

	if ic.CertPath != "" {
		writeChainErr = peertls.WriteChain(&certData, chain...)
		writeChainDataErr = writeChainData(ic.CertPath, certData.Bytes())
	}

	writeErr := utils.CombineErrors(writeChainErr)
	if writeErr != nil {
		return writeErr
	}

	return utils.CombineErrors(
		writeChainDataErr,
	)
}

// SaveBackup saves the certificate of the config with a timestamped filename
func (ic PeerConfig) SaveBackup(pi *PeerIdentity) error {
	return PeerConfig{
		CertPath: backupPath(ic.CertPath),
	}.Save(pi)
}

// ChainRaw returns all of the certificate chain as a 2d byte slice
func (fi *FullIdentity) ChainRaw() [][]byte {
	chain := [][]byte{fi.Leaf.Raw, fi.CA.Raw}
	for _, cert := range fi.RestChain {
		chain = append(chain, cert.Raw)
	}
	return chain
}

// RestChainRaw returns the rest (excluding leaf and CA) of the certificate chain as a 2d byte slice
func (fi *FullIdentity) RestChainRaw() [][]byte {
	var chain [][]byte
	for _, cert := range fi.RestChain {
		chain = append(chain, cert.Raw)
	}
	return chain
}

// PeerIdentity converts a FullIdentity into a PeerIdentity
func (fi *FullIdentity) PeerIdentity() *PeerIdentity {
	return &PeerIdentity{
		CA:        fi.CA,
		Leaf:      fi.Leaf,
		ID:        fi.ID,
		RestChain: fi.RestChain,
	}
}

func backupPath(path string) string {
	pathExt := filepath.Ext(path)
	base := strings.TrimSuffix(path, pathExt)
	return fmt.Sprintf(
		"%s.%s%s",
		base,
		strconv.Itoa(int(time.Now().Unix())),
		pathExt,
	)
}
