// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/peertls"
)

// Creds implements dht.nodeID and is used for the private portion of an identity (i.e. tls cert/private key)
type Creds struct {
	hash    []byte
	hashLen uint16
	tlsH    *peertls.TLSHelper
}

// LoadID reads and parses an "identity" file containing a tls certificate
// chain (leaf-first), private key, and "id options" for the "identity file"
// at `path`.
//
// The "identity file" must contain PEM encoded data. The certificate portion
// may contain intermediate certificates following the leaf certificate to
// form a certificate chain.
func LoadID(path string, hashLen uint16) (*Creds, error) {
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

	cert, err := read(IDBytes)
	kadCreds, err := CertToCreds(cert, hashLen)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return kadCreds, nil
}

// NewID returns a pointer to a newly intialized, NodeID with at least the
// given difficulty
func NewID(difficulty uint16, hashLen uint16, concurrency uint) (dht.NodeID, error) {
	done := make(chan bool, 0)
	c := make(chan Creds, 1)
	for i := 0; i < int(concurrency); i++ {
		go generateCreds(difficulty, hashLen, c, done)
	}

	creds, _ := <-c
	close(done)

	return &creds, nil
}

// Save saves the certificate chain (leaf-first), private key, and
// hash length (ordered respectively) from `Creds` to a single
// PEM-encoded "identity file".
func (c *Creds) Save(path string) error {
	baseDir := filepath.Dir(path)

	if err := os.MkdirAll(baseDir, 600); err != nil {
		return errs.Wrap(err)
	}

	c.writeRootKey(baseDir)

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errs.New("unable to open identity file for writing \"%s\"", path, err)
	}

	defer func() {
		if err := file.Close(); err != nil {
			zap.S().Error(errs.Wrap(err))
		}
	}()

	if err = c.write(file); err != nil {
		return err
	}

	return nil
}

// CertToCreds takes a tls certificate pointer and a hash length to build a `Creds` struct
func CertToCreds(cert *tls.Certificate, hashLen uint16) (*Creds, error) {
	pubKey, ok := cert.Leaf.PublicKey.(*ecdsa.PublicKey)
	if pubKey == nil || !ok {
		return nil, errs.New("unsupported public key type (type assertion to `*ecdsa.PublicKey` failed)")
	}

	keyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, errs.New("unable to marshal pubKey", err)
	}

	hashBytes, err := hash(keyBytes, hashLen)
	if err != nil {
		return nil, err
	}

	tlsH, err := peertls.NewTLSHelper(cert)
	if err != nil {
		return nil, err
	}

	kadCreds := &Creds{
		tlsH:    tlsH,
		hash:    hashBytes,
		hashLen: hashLen,
	}

	kadCreds.tlsH.BaseConfig = baseConfig(kadCreds.Difficulty(), hashLen)

	return kadCreds, nil
}

// String serializes the hash, public key, and hash length into a PEM-encoded string
func (c *Creds) String() string {
	return string(c.Bytes())
}

// Bytes serializes the hash, public key, and hash length into a PEM-encoded byte-slice
func (c *Creds) Bytes() []byte {
	p := c.tlsH.PubKey()
	pubKey, err := x509.MarshalPKIXPublicKey(&p)
	if err != nil {
		zap.S().Error(errs.New("unable to marshal public key"))
	}

	return idBytes(c.hash, pubKey, c.hashLen)
}

// Hash returns the hash the public key to a langth of `k.hashLen`
func (c *Creds) Hash() []byte {
	return c.hash
}

// Difficulty returns the number of trailing zero-value bits in the hash
func (c *Creds) Difficulty() uint16 {
	return idDifficulty(c.Hash())
}
