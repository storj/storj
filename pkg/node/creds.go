// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/peertls"
)

// Creds implements dht.nodeID and is used for the private portion of an identity (i.e. tls cert/private key)
type Creds struct {
	hash    []byte
	hashLen uint16
	tlsH    *peertls.TLSHelper
}

// Save saves the certificate chain (leaf-first), private key, and
// hash length (ordered respectively) from `Creds` to a single
// PEM-encoded "identity file".
func (c *Creds) Save(path string) error {
	baseDir := filepath.Dir(path)

	if _, err := os.Stat(baseDir); err != nil {
		if err == os.ErrNotExist {
			if err := os.MkdirAll(baseDir, 600); err != nil {
				return errs.Wrap(err)
			}
		} else {
			return errs.Wrap(err)
		}
	}

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

	b := bytes.NewBuffer([]byte{})
	encoder := base64.NewEncoder(base64.URLEncoding, b)
	if _, err := encoder.Write(c.hash); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if _, err := encoder.Write(pubKey); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if err := binary.Write(encoder, binary.LittleEndian, c.hashLen); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if err := encoder.Close(); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	return b.Bytes()
}

// Hash returns the hash the public key to a langth of `k.hashLen`
func (c *Creds) Hash() []byte {
	return c.hash
}

// Difficulty returns the number of trailing zero-value bytes in the hash
func (c *Creds) Difficulty() uint16 {
	hash := c.Hash()
	for i := 1; i < len(hash); i++ {
		b := hash[len(hash)-i]

		if b != 0 {
			return uint16(i - 1)
		}
	}

	// NB: this should never happen
	return 0
}
