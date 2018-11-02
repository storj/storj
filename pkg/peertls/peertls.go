// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"io"

	"github.com/zeebo/errs"
)

const (
	// BlockTypeEcPrivateKey is the value to define a block type of private key
	BlockTypeEcPrivateKey = "EC PRIVATE KEY"
	// BlockTypeCertificate is the value to define a block type of certificate
	BlockTypeCertificate = "CERTIFICATE"
	// BlockTypeIDOptions is the value to define a block type of id options
	// (e.g. `version`)
	BlockTypeIDOptions = "ID OPTIONS"
)

var (
	// AuthoritySignatureExtID is the asn1 object ID for a pkix extension holding a signature of the leaf cert, signed by some CA (e.g. the root cert)
	// This extension allows for an additional signature per certificate
	AuthoritySignatureExtID = asn1.ObjectIdentifier{2, 999, 1}
	// ErrNotExist is used when a file or directory doesn't exist
	ErrNotExist = errs.Class("file or directory not found error")
	// ErrGenerate is used when an error occurred during cert/key generation
	ErrGenerate = errs.Class("tls generation error")
	// ErrUnsupportedKey is used when key type is not supported
	ErrUnsupportedKey = errs.Class("unsupported key type")
	// ErrTLSTemplate is used when an error occurs during tls template generation
	ErrTLSTemplate = errs.Class("tls template error")
	// ErrVerifyPeerCert is used when an error occurs during `VerifyPeerCertificate`
	ErrVerifyPeerCert = errs.Class("tls peer certificate verification error")
	// ErrParseCerts is used when an error occurs while parsing a certificate or cert chain
	ErrParseCerts = errs.Class("unable to parse certificate")
	// ErrVerifySignature is used when a cert-chain signature verificaion error occurs
	ErrVerifySignature = errs.Class("tls certificate signature verification error")
	// ErrVerifyCertificateChain is used when a certificate chain can't be verified from leaf to root
	// (i.e.: each cert in the chain should be signed by the preceding cert and the root should be self-signed)
	ErrVerifyCertificateChain = errs.Class("certificate chain signature verification failed")
	// ErrVerifyCAWhitelist is used when the leaf of a peer certificate isn't signed by any CA in the whitelist
	ErrVerifyCAWhitelist = errs.Class("certificate isn't signed by any CA in the whitelist")
	// ErrSign is used when something goes wrong while generating a signature
	ErrSign = errs.Class("unable to generate signature")
)

// PeerCertVerificationFunc is the signature for a `*tls.Config{}`'s
// `VerifyPeerCertificate` function.
type PeerCertVerificationFunc func([][]byte, [][]*x509.Certificate) error

// NewKey returns a new PrivateKey
func NewKey() (crypto.PrivateKey, error) {
	k, err := ecdsa.GenerateKey(authECCurve, rand.Reader)
	if err != nil {
		return nil, ErrGenerate.New("failed to generate private key: %v", err)
	}

	return k, nil
}

// NewCert returns a new x509 certificate using the provided templates and
// signed by the `signer` key
func NewCert(template, parentTemplate *x509.Certificate, pubKey crypto.PublicKey, signer crypto.PrivateKey) (*x509.Certificate, error) {
	k, ok := signer.(*ecdsa.PrivateKey)
	if !ok {
		return nil, ErrUnsupportedKey.New("%T", k)
	}

	if parentTemplate == nil {
		parentTemplate = template
	}

	cb, err := x509.CreateCertificate(
		rand.Reader,
		template,
		parentTemplate,
		pubKey,
		k,
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	c, err := x509.ParseCertificate(cb)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return c, nil
}

// VerifyPeerFunc combines multiple `*tls.Config#VerifyPeerCertificate`
// functions and adds certificate parsing.
func VerifyPeerFunc(next ...PeerCertVerificationFunc) PeerCertVerificationFunc {
	return func(chain [][]byte, _ [][]*x509.Certificate) error {
		c, err := parseCertificateChains(chain)
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
// which are signed by their respective parents, ending with a self-signed root
func VerifyPeerCertChains(_ [][]byte, parsedChains [][]*x509.Certificate) error {
	return verifyChainSignatures(parsedChains[0])
}

// VerifyCAWhitelist verifies that the peer identity's CA and leaf-extension was signed
// by any one of the (certificate authority) certificates in the provided whitelist
func VerifyCAWhitelist(cas []*x509.Certificate, verifyExtension bool) PeerCertVerificationFunc {
	if cas == nil {
		return nil
	}

	return func(_ [][]byte, parsedChains [][]*x509.Certificate) error {
		var (
			leaf = parsedChains[0][0]
			err  error
		)

		// Leaf extension must contain leaf signature, signed by a CA in the whitelist.
		// That *same* CA must also have signed the leaf's parent cert (regular cert chain signature, not extension).
		for _, ca := range cas {
			err = verifyCertSignature(ca, parsedChains[0][1])
			if err == nil {
				if !verifyExtension {
					break
				}

				for _, ext := range leaf.Extensions {
					if ext.Id.Equal(AuthoritySignatureExtID) {
						err = verifySignature(ext.Value, leaf.RawTBSCertificate, leaf.PublicKey)
						if err != nil {
							return ErrVerifyCAWhitelist.New("authority signature extension verification error: %s", err.Error())
						}
						return nil
					}
				}
				break
			}
		}

		return ErrVerifyCAWhitelist.Wrap(err)
	}
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

// TLSCert creates a tls.Certificate from chains, key and leaf
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

	for _, c := range chain {
		if err := pem.Encode(w, NewCertBlock(c.Raw)); err != nil {
			return errs.Wrap(err)
		}
	}
	return nil
}

// WriteKey writes the private key to the writer, PEM-encoded.
func WriteKey(w io.Writer, key crypto.PrivateKey) error {
	var (
		kb  []byte
		err error
	)

	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		kb, err = x509.MarshalECPrivateKey(k)
		if err != nil {
			return errs.Wrap(err)
		}
	default:
		return ErrUnsupportedKey.New("%T", k)
	}

	if err := pem.Encode(w, NewKeyBlock(kb)); err != nil {
		return errs.Wrap(err)
	}
	return nil
}
