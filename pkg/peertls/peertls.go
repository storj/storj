// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"io"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/utils"
)

var (
	// ErrNotExist is used when a file or directory doesn't exist.
	ErrNotExist = errs.Class("file or directory not found error")
	// ErrTLSTemplate is used when an error occurs during tls template generation.
	ErrTLSTemplate = errs.Class("tls template error")
	// ErrGenerate is used when an error occurred during cert/key generation.
	ErrGenerate = errs.Class("tls generation error")
	// ErrVerifyPeerCert is used when an error occurs during `VerifyPeerCertificate`.
	ErrVerifyPeerCert = errs.Class("tls peer certificate verification error")
	// ErrVerifyCertificateChain is used when a certificate chain can't be verified from leaf to root
	// (i.e.: each cert in the chain should be signed by the preceding cert and the root should be self-signed).
	ErrVerifyCertificateChain = errs.Class("certificate chain signature verification failed")
	// ErrVerifyCAWhitelist is used when a signature wasn't produced by any CA in the whitelist.
	ErrVerifyCAWhitelist = errs.Class("not signed by any CA in the whitelist")
)

// PeerCertVerificationFunc is the signature for a `*tls.Config{}`'s
// `VerifyPeerCertificate` function.
type PeerCertVerificationFunc func([][]byte, [][]*x509.Certificate) error

// VerifyPeerFunc combines multiple `*tls.Config#VerifyPeerCertificate`
// functions and adds certificate parsing.
func VerifyPeerFunc(next ...PeerCertVerificationFunc) PeerCertVerificationFunc {
	return func(chain [][]byte, _ [][]*x509.Certificate) error {
		c, err := pkcrypto.ParseCertificates(chain)
		if err != nil {
			return ErrVerifyPeerCert.Wrap(err)
		}

		for _, n := range next {
			if n != nil {
				if err := n(chain, [][]*x509.Certificate{c}); err != nil {
					return ErrVerifyPeerCert.Wrap(err)
				}
			}
		}
		return nil
	}
}

// VerifyPeerCertChains verifies that the first certificate chain contains certificates
// which are signed by their respective parents, ending with a self-signed root.
func VerifyPeerCertChains(_ [][]byte, parsedChains [][]*x509.Certificate) error {
	return verifyChainSignatures(parsedChains[0])
}

// VerifyCAWhitelist verifies that the peer identity's CA was signed by any one
// of the (certificate authority) certificates in the provided whitelist.
func VerifyCAWhitelist(cas []*x509.Certificate) PeerCertVerificationFunc {
	if cas == nil {
		return nil
	}
	return func(_ [][]byte, parsedChains [][]*x509.Certificate) error {
		for _, ca := range cas {
			err := verifyCertSignature(ca, parsedChains[0][CAIndex])
			if err == nil {
				return nil
			}
		}
		return ErrVerifyCAWhitelist.New("CA cert")
	}
}

// TLSCert creates a tls.Certificate from chains, key and leaf.
func TLSCert(chain [][]byte, leaf *x509.Certificate, key crypto.PrivateKey) (*tls.Certificate, error) {
	var err error
	if leaf == nil {
		leaf, err = x509.ParseCertificate(chain[0])
		if err != nil {
			return nil, err
		}
	}

	return &tls.Certificate{
		Leaf:        leaf,
		Certificate: chain,
		PrivateKey:  key,
	}, nil
}

// WriteChain writes the certificate chain (leaf-first) to the writer, PEM-encoded.
func WriteChain(w io.Writer, chain ...*x509.Certificate) error {
	if len(chain) < 1 {
		return errs.New("expected at least one certificate for writing")
	}

	var extErrs utils.ErrorGroup
	for _, c := range chain {
		if err := pem.Encode(w, pkcrypto.NewCertBlock(c.Raw)); err != nil {
			return errs.Wrap(err)
		}

		for _, e := range c.ExtraExtensions {
			extBytes, err := asn1.Marshal(e)
			if err != nil {
				extErrs.Add(errs.Wrap(err))
			}
			if err := pem.Encode(w, pkcrypto.NewExtensionBlock(extBytes)); err != nil {
				extErrs.Add(errs.Wrap(err))
			}
		}
	}
	return extErrs.Finish()
}

// ChainBytes returns bytes of the certificate chain (leaf-first) to the writer, PEM-encoded.
func ChainBytes(chain ...*x509.Certificate) ([]byte, error) {
	var data bytes.Buffer
	err := WriteChain(&data, chain...)
	return data.Bytes(), err
}

// NewCert returns a new x509 certificate using the provided templates and key,
// signed by the parent cert if provided; otherwise, self-signed.
func NewCert(key, parentKey crypto.PrivateKey, template, parent *x509.Certificate) (*x509.Certificate, error) {
	p, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, pkcrypto.ErrUnsupportedKey.New("%T", key)
	}

	var signingKey crypto.PrivateKey
	if parentKey != nil {
		signingKey = parentKey
	} else {
		signingKey = key
	}

	if parent == nil {
		parent = template
	}

	cb, err := x509.CreateCertificate(
		rand.Reader,
		template,
		parent,
		&p.PublicKey,
		signingKey,
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	cert, err := x509.ParseCertificate(cb)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return cert, nil
}
