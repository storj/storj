// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"encoding/gob"
	"time"

	"github.com/zeebo/errs"
)

const (
	// SignedCertExtID is the asn1 object ID for a pkix extensionHandler holding a
	// signature of the cert it's extending, signed by some CA (e.g. the root cert chain).
	// This extensionHandler allows for an additional signature per certificate.
	SignedCertExtID = iota
	// RevocationExtID is the asn1 object ID for a pkix extensionHandler containing the
	// most recent certificate revocation data
	// for the current TLS cert chain.
	RevocationExtID
	// RevocationBucket is the bolt bucket to store revocation data in
	RevocationBucket = "revocations"
)

const (
	// LeafIndex is the index of the leaf certificate in a cert chain (0)
	LeafIndex = iota
	// CAIndex is the index of the CA certificate in a cert chain (1)
	CAIndex
)

var (
	// ExtensionIDs is a map from an enum to object identifiers used in peertls extensions.
	ExtensionIDs = map[int]asn1.ObjectIdentifier{
		// NB: 2.999.X is reserved for "example" OIDs
		// (see http://oid-info.com/get/2.999)
		SignedCertExtID: {2, 999, 1, 1},
		RevocationExtID: {2, 999, 1, 2},
	}
	// ErrExtension is used when an error occurs while processing an extensionHandler.
	ErrExtension = errs.Class("extension error")
	// ErrRevocation is used when an error occurs involving a certificate revocation
	ErrRevocation = errs.Class("revocation processing error")
	// ErrRevocationDB is used when an error occurs involving the revocations database
	ErrRevocationDB = errs.Class("revocation database error")
	// ErrRevokedCert is used when a certificate in the chain is revoked and not expected to be
	ErrRevokedCert = ErrRevocation.New("a certificate in the chain is revoked")
	// ErrUniqueExtensions is used when multiple extensions have the same Id
	ErrUniqueExtensions = ErrExtension.New("extensions are not unique")
	// ErrRevocationTimestamp is used when a revocation's timestamp is older than the last recorded revocation
	ErrRevocationTimestamp = ErrExtension.New("revocation timestamp is older than last known revocation")
)

// TLSExtConfig is used to bind cli flags for determining which extensions will
// be used by the server
type TLSExtConfig struct {
	Revocation          bool `help:"if true, client leaves may contain the most recent certificate revocation for the current certificate" default:"true"`
	WhitelistSignedLeaf bool `help:"if true, client leaves must contain a valid \"signed certificate extension\" (NB: verified against certs in the peer ca whitelist; i.e. if true, a whitelist must be provided)" default:"false"`
}

// ExtensionHandlers is a collection of `extensionHandler`s for convenience (see `VerifyFunc`)
type ExtensionHandlers []ExtensionHandler

type extensionVerificationFunc func(pkix.Extension, [][]*x509.Certificate) error

// ExtensionHandler represents a verify function for handling an extension
// with the given ID
type ExtensionHandler struct {
	ID     asn1.ObjectIdentifier
	Verify extensionVerificationFunc
}

// ParseExtOptions holds options for calling `ParseExtensions`
type ParseExtOptions struct {
	CAWhitelist []*x509.Certificate
	RevDB       RevocationDB
}

// Revocation represents a certificate revocation for storage in the revocation
// database and for use in a TLS extension
type Revocation struct {
	Timestamp int64
	CertHash  []byte
	Signature []byte
}

// RevocationDB stores certificate revocation data.
type RevocationDB interface {
	Get(chain []*x509.Certificate) (*Revocation, error)
	Put(chain []*x509.Certificate, ext pkix.Extension) error
	List() ([]*Revocation, error)
	Close() error
}

// ParseExtensions parses an extension config into a slice of extension handlers
// with their respective ids (`asn1.ObjectIdentifier`) and a "verify" function
// to be used in the context of peer certificate verification.
func ParseExtensions(c TLSExtConfig, opts ParseExtOptions) (handlers ExtensionHandlers) {
	if c.WhitelistSignedLeaf {
		handlers = append(handlers, ExtensionHandler{
			ID:     ExtensionIDs[SignedCertExtID],
			Verify: verifyCAWhitelistSignedLeafFunc(opts.CAWhitelist),
		})
	}

	if c.Revocation {
		handlers = append(handlers, ExtensionHandler{
			ID: ExtensionIDs[RevocationExtID],
			Verify: func(certExt pkix.Extension, chains [][]*x509.Certificate) error {
				if err := opts.RevDB.Put(chains[0], certExt); err != nil {
					return err
				}
				return nil
			},
		})
	}

	return handlers
}

// NewRevocationExt generates a revocation extension for a certificate.
func NewRevocationExt(key crypto.PrivateKey, revokedCert *x509.Certificate) (pkix.Extension, error) {
	nowUnix := time.Now().Unix()

	hash, err := SHA256Hash(revokedCert.Raw)
	if err != nil {
		return pkix.Extension{}, err
	}
	rev := Revocation{
		Timestamp: nowUnix,
		CertHash:  make([]byte, len(hash)),
	}
	copy(rev.CertHash, hash)

	if err := rev.Sign(key); err != nil {
		return pkix.Extension{}, err
	}

	revBytes, err := rev.Marshal()
	if err != nil {
		return pkix.Extension{}, err
	}

	ext := pkix.Extension{
		Id:    ExtensionIDs[RevocationExtID],
		Value: make([]byte, len(revBytes)),
	}
	copy(ext.Value, revBytes)

	return ext, nil
}

// AddRevocationExt generates a revocation extension for a cert and attaches it
// to the cert which will replace the revoked cert.
func AddRevocationExt(key crypto.PrivateKey, revokedCert, newCert *x509.Certificate) error {
	ext, err := NewRevocationExt(key, revokedCert)
	if err != nil {
		return err
	}
	err = AddExtension(newCert, ext)
	if err != nil {
		return err
	}
	return nil
}

// AddSignedCertExt generates a signed certificate extension for a cert and attaches
// it to that cert.
func AddSignedCertExt(key crypto.PrivateKey, cert *x509.Certificate) error {
	signature, err := signHashOf(key, cert.RawTBSCertificate)
	if err != nil {
		return err
	}

	err = AddExtension(cert, pkix.Extension{
		Id:    ExtensionIDs[SignedCertExtID],
		Value: signature,
	})
	if err != nil {
		return err
	}
	return nil
}

// AddExtension adds one or more extensions to a certificate
func AddExtension(cert *x509.Certificate, exts ...pkix.Extension) (err error) {
	if len(exts) == 0 {
		return nil
	}
	if !uniqueExts(append(cert.ExtraExtensions, exts...)) {
		return ErrUniqueExtensions
	}

	for _, ext := range exts {
		e := pkix.Extension{Id: ext.Id, Value: make([]byte, len(ext.Value))}
		copy(e.Value, ext.Value)
		cert.ExtraExtensions = append(cert.ExtraExtensions, e)
	}
	return nil
}

// VerifyFunc returns a peer certificate verification function which iterates
// over all the leaf cert's extensions and receiver extensions and calls
// `extensionHandler#verify` when it finds a match by id (`asn1.ObjectIdentifier`)
func (e ExtensionHandlers) VerifyFunc() PeerCertVerificationFunc {
	if len(e) == 0 {
		return nil
	}
	return func(_ [][]byte, parsedChains [][]*x509.Certificate) error {
		leafExts := make(map[string]pkix.Extension)
		for _, ext := range parsedChains[0][LeafIndex].ExtraExtensions {
			leafExts[ext.Id.String()] = ext
		}

		for _, handler := range e {
			if ext, ok := leafExts[handler.ID.String()]; ok {
				err := handler.Verify(ext, parsedChains)
				if err != nil {
					return ErrExtension.Wrap(err)
				}
			}
		}
		return nil
	}
}

// Verify checks if the signature of the revocation was produced by the passed cert's public key.
func (r Revocation) Verify(signingCert *x509.Certificate) error {
	pubKey, ok := signingCert.PublicKey.(crypto.PublicKey)
	if !ok {
		return ErrUnsupportedKey.New("%T", signingCert.PublicKey)
	}

	data, err := r.TBSBytes()
	if err != nil {
		return err
	}

	if err := VerifySignature(r.Signature, data, pubKey); err != nil {
		return err
	}
	return nil
}

// TBSBytes (ToBeSigned) returns the hash of the revoked certificate hash and
// the timestamp (i.e. hash(hash(cert bytes) + timestamp)).
func (r *Revocation) TBSBytes() ([]byte, error) {
	toHash := new(bytes.Buffer)
	_, err := toHash.Write(r.CertHash)
	if err != nil {
		return nil, ErrExtension.Wrap(err)
	}

	// NB: append timestamp to revoked cert bytes
	binary.PutVarint(toHash.Bytes(), r.Timestamp)

	return SHA256Hash(toHash.Bytes())
}

// Sign generates a signature using the passed key and attaches it to the revocation.
func (r *Revocation) Sign(key crypto.PrivateKey) error {
	data, err := r.TBSBytes()
	if err != nil {
		return err
	}

	r.Signature, err = signHashOf(key, data)
	if err != nil {
		return err
	}
	return nil
}

// Marshal serializes a revocation to bytes
func (r Revocation) Marshal() ([]byte, error) {
	data := new(bytes.Buffer)
	// NB: using gob instead of asn1 because we plan to leave tls and asn1 is meh
	encoder := gob.NewEncoder(data)
	err := encoder.Encode(r)
	if err != nil {
		return nil, err
	}

	return data.Bytes(), nil
}

// Unmarshal deserializes a revocation from bytes
func (r *Revocation) Unmarshal(data []byte) error {
	// NB: using gob instead of asn1 because we plan to leave tls and asn1 is meh
	decoder := gob.NewDecoder(bytes.NewBuffer(data))
	if err := decoder.Decode(r); err != nil {
		return err
	}
	return nil
}

func verifyCAWhitelistSignedLeafFunc(caWhitelist []*x509.Certificate) extensionVerificationFunc {
	return func(certExt pkix.Extension, chains [][]*x509.Certificate) error {
		if caWhitelist == nil {
			return ErrVerifyCAWhitelist.New("no whitelist provided")
		}

		leaf := chains[0][LeafIndex]
		for _, ca := range caWhitelist {
			err := VerifySignature(certExt.Value, leaf.RawTBSCertificate, ca.PublicKey)
			if err == nil {
				return nil
			}
		}
		return ErrVerifyCAWhitelist.New("leaf extension")
	}
}
