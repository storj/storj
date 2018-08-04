// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto/ecdsa"
	"crypto/x509"
	"github.com/zeebo/errs"
	"crypto/tls"
	"crypto/rand"
	"encoding/pem"
	"reflect"
)

const (
	// BlockTypeEcPrivateKey is the value to define a block type of private key
	BlockTypeEcPrivateKey = "EC PRIVATE KEY"
	// BlockTypeCertificate is the value to define a block type of certificate
	BlockTypeCertificate = "CERTIFICATE"
	// BlockTypeIDOptions is the value to define a block type of id options
	// (e.g. `version`)
	// BlockTypeIDOptions = "ID OPTIONS"
)

var (
	// ErrNotExist is used when a file or directory doesn't exist
	ErrNotExist = errs.Class("file or directory not found error")
	// ErrGenerate is used when an error occured during cert/key generation
	ErrGenerate = errs.Class("tls generation error")
	// ErrTLSOptions is used inconsistently and should probably just be removed
	ErrUnsupportedKey = errs.Class("unsupported key type")
	// ErrTLSTemplate is used when an error occurs during tls template generation
	ErrTLSTemplate = errs.Class("tls template error")
	// ErrVerifyPeerCert is used when an error occurs during `VerifyPeerCertificate`
	ErrVerifyPeerCert = errs.Class("tls peer certificate verification error")
	// ErrVerifySignature is used when a cert-chain signature verificaion error occurs
	ErrVerifySignature = errs.Class("tls certificate signature verification error")
)

// PeerCertVerificationFunc is the signature for a `*tls.Config{}`'s
// `VerifyPeerCertificate` function.
type PeerCertVerificationFunc func([][]byte, [][]*x509.Certificate) error

func Generate(template, parentTemplate *x509.Certificate, parent, signer *tls.Certificate) (*tls.Certificate, error) {
	k, err := ecdsa.GenerateKey(authECCurve, rand.Reader)
	if err != nil {
		return nil, ErrGenerate.New("failed to generateServerTLS root private key", err)
	}

	if signer == nil {
		signer = &tls.Certificate{
			PrivateKey: k,
			Leaf: &x509.Certificate{
				PublicKey: &k.PublicKey,
			},
		}
	}
	sk, ok := signer.Leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, ErrUnsupportedKey.New("%s", reflect.TypeOf(sk))
	}

	if parent == nil {
		parent = &tls.Certificate{
			Certificate: nil,
		}
	}
	if parentTemplate == nil {
		parentTemplate = template
	}
	caCert, err := createCert(
		template,
		parentTemplate,
		parent.Certificate,
		sk,
		k,
		signer.PrivateKey,
	)
	if err != nil {
		return nil, ErrGenerate.Wrap(err)
	}

	return caCert, nil
}

// VerifyPeerFunc combines multiple `*tls.Config#VerifyPeerCertificate`
// functions and adds certificate parsing.
func VerifyPeerFunc(next ...PeerCertVerificationFunc) PeerCertVerificationFunc {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		parsedCerts, err := parseCertificateChains(rawCerts)
		if err != nil {
			return err
		}

		for _, n := range next {
			if n != nil {
				if err := n(nil, [][]*x509.Certificate{parsedCerts}); err != nil {
					return err
				}
			}
		}

		return nil
	}
}

func VerifyPeerCertChains(_ [][]byte, parsedChains [][]*x509.Certificate) error {
	return verifyChainSignatures(parsedChains[0])
}

// NewKeyBlock converts an ASN1/DER-encoded byte-slice of a private key into
// a `pem.Block` pointer
func NewKeyBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeEcPrivateKey, Bytes: b}
}

// NewCertBlock converts an ASN1/DER-encoded byte-slice of a tls certificate
// into a `pem.Block` pointer
func NewCertBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeCertificate, Bytes: b}
}
