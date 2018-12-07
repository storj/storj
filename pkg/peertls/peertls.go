// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
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

const (
	// SignedCertExtID is the asn1 object ID for a pkix extension holding a signature of the cert it's extending, signed by some CA (e.g. the root cert chain).
	// This extension allows for an additional signature per certificate.
	SignedCertExtID = iota
	// RevocationExtID is the asn1 object ID for a pkix extension containing the most recent certificate revocation data
	// for the current TLS cert chain.
	RevocationExtID
)

var (
	// ExtensionIDs is a map from an enum to extension object identifiers.
	ExtensionIDs = map[int]asn1.ObjectIdentifier{
		SignedCertExtID: {2, 999, 1, 1},
		RevocationExtID: {2, 999, 2, 1},
	}
	// ErrExtension is used when an error occurs while processing an extension.
	ErrExtension = errs.Class("extension error")
	// ErrNotExist is used when a file or directory doesn't exist.
	ErrNotExist = errs.Class("file or directory not found error")
	// ErrGenerate is used when an error occurred during cert/key generation.
	ErrGenerate = errs.Class("tls generation error")
	// ErrUnsupportedKey is used when key type is not supported.
	ErrUnsupportedKey = errs.Class("unsupported key type")
	// ErrTLSTemplate is used when an error occurs during tls template generation.
	ErrTLSTemplate = errs.Class("tls template error")
	// ErrVerifyPeerCert is used when an error occurs during `VerifyPeerCertificate`.
	ErrVerifyPeerCert = errs.Class("tls peer certificate verification error")
	// ErrParseCerts is used when an error occurs while parsing a certificate or cert chain.
	ErrParseCerts = errs.Class("unable to parse certificate")
	// ErrVerifySignature is used when a cert-chain signature verificaion error occurs.
	ErrVerifySignature = errs.Class("tls certificate signature verification error")
	// ErrVerifyCertificateChain is used when a certificate chain can't be verified from leaf to root
	// (i.e.: each cert in the chain should be signed by the preceding cert and the root should be self-signed).
	ErrVerifyCertificateChain = errs.Class("certificate chain signature verification failed")
	// ErrVerifyCAWhitelist is used when the leaf of a peer certificate isn't signed by any CA in the whitelist.
	ErrVerifyCAWhitelist = errs.Class("not signed by any CA in the whitelist")
	// ErrSign is used when something goes wrong while generating a signature.
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
// which are signed by their respective parents, ending with a self-signed root.
func VerifyPeerCertChains(_ [][]byte, parsedChains [][]*x509.Certificate) error {
	return verifyChainSignatures(parsedChains[0])
}

// VerifyCAWhitelist verifies that the peer identity's CA and leaf-extension was signed
// by any one of the (certificate authority) certificates in the provided whitelist.
func VerifyCAWhitelist(cas []*x509.Certificate) PeerCertVerificationFunc {
	if cas == nil {
		return nil
	}
	return func(_ [][]byte, parsedChains [][]*x509.Certificate) error {
		for _, ca := range cas {
			err := verifyCertSignature(ca, parsedChains[0][1])
			if err == nil {
				return nil
			}
		}
		return ErrVerifyCAWhitelist.New("extension signature doesn't match any CA in the whitelist")
	}
}

// NewKeyBlock converts an ASN1/DER-encoded byte-slice of a private key into
// a `pem.Block` pointer.
func NewKeyBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeEcPrivateKey, Bytes: b}
}

// NewCertBlock converts an ASN1/DER-encoded byte-slice of a tls certificate
// into a `pem.Block` pointer.
func NewCertBlock(b []byte) *pem.Block {
	return &pem.Block{Type: BlockTypeCertificate, Bytes: b}
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

// NewCert returns a new x509 certificate using the provided templates and key,
// signed by the parent cert if provided; otherwise, self-signed.
func NewCert(key, parentKey crypto.PrivateKey, template, parent *x509.Certificate) (*x509.Certificate, error) {
	p, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, ErrUnsupportedKey.New("%T", key)
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

// AddSignedLeafExt adds a "signed certificate extension" to the passed cert,
// using the passed private key.
func AddSignedLeafExt(key crypto.PrivateKey, cert *x509.Certificate) error {
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return ErrUnsupportedKey.New("%T", key)
	}

	hash := crypto.SHA256.New()
	_, err := hash.Write(cert.RawTBSCertificate)
	if err != nil {
		return ErrSign.Wrap(err)
	}
	r, s, err := ecdsa.Sign(rand.Reader, ecKey, hash.Sum(nil))
	if err != nil {
		return ErrSign.Wrap(err)
	}

	signature, err := asn1.Marshal(ECDSASignature{R: r, S: s})
	if err != nil {
		return ErrSign.Wrap(err)
	}

	cert.ExtraExtensions = append(cert.ExtraExtensions, pkix.Extension{
		Id:    ExtensionIDs[SignedCertExtID],
		Value: signature,
	})
	return nil
}
