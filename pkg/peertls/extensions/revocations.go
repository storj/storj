// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package extensions

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/gob"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/pkcrypto"
)

var (
	// RevocationCheckHandler ensures that a remote peer's certificate chain
	// doesn't contain any revoked certificates.
	RevocationCheckHandler = NewHandlerFactory(&RevocationExtID, revocationChecker)
	// RevocationUpdateHandler looks for certificate revocation extensions on a
	// remote peer's certificate chain, adding them to the revocation DB if valid.
	RevocationUpdateHandler = NewHandlerFactory(&RevocationExtID, revocationUpdater)
)

// ErrRevocation is used when an error occurs involving a certificate revocation
var ErrRevocation = errs.Class("revocation processing error")

// ErrRevocationDB is used when an error occurs involving the revocations database
var ErrRevocationDB = errs.Class("revocation database error")

// ErrRevokedCert is used when a certificate in the chain is revoked and not expected to be
var ErrRevokedCert = ErrRevocation.New("a certificate in the chain is revoked")

// ErrRevocationTimestamp is used when a revocation's timestamp is older than the last recorded revocation
var ErrRevocationTimestamp = Error.New("revocation timestamp is older than last known revocation")

// Revocation represents a certificate revocation for storage in the revocation
// database and for use in a TLS extension.
type Revocation struct {
	Timestamp int64
	CertHash  []byte
	Signature []byte
	Final     bool
}

// RevocationDB stores certificate revocation data.
type RevocationDB interface {
	Get(chain []*x509.Certificate) (*Revocation, error)
	Put(chain []*x509.Certificate, ext pkix.Extension) error
	List() ([]*Revocation, error)
	Close() error
}

func init() {
	// NB: register all handlers defined in this file.
	AllHandlers.Register(
		RevocationCheckHandler,
		RevocationUpdateHandler,
	)
}

// NewRevocationExt generates a revocation extension for a certificate.
func NewRevocationExt(key crypto.PrivateKey, revokedCert *x509.Certificate, final bool) (pkix.Extension, error) {
	nowUnix := time.Now().Unix()

	hash := pkcrypto.SHA256Hash(revokedCert.Raw)
	rev := Revocation{
		Timestamp: nowUnix,
		CertHash:  hash,
		Final:     final,
	}

	if err := rev.Sign(key); err != nil {
		return pkix.Extension{}, err
	}

	revBytes, err := rev.Marshal()
	if err != nil {
		return pkix.Extension{}, err
	}

	ext := pkix.Extension{
		Id:    RevocationExtID,
		Value: revBytes,
	}

	return ext, nil
}

func revocationChecker(opts *Options) HandlerFunc {
	return func(_ pkix.Extension, chains [][]*x509.Certificate) error {
		leaf := chains[0][peertls.LeafIndex]
		lastRev, lastRevErr := opts.RevDB.Get(chains[0])
		if lastRevErr != nil {
			return Error.Wrap(lastRevErr)
		}
		if lastRev == nil {
			return nil
		}

		leafHash := pkcrypto.SHA256Hash(leaf.Raw)

		// NB: we trust that anything that made it into the revocation DB is valid
		//		(i.e. no need for further verification)
		switch {
		case lastRev.Final:
			fallthrough
		case bytes.Equal(lastRev.CertHash, leafHash):
			return ErrRevokedCert
		default:
			return nil
		}
	}
}

func revocationUpdater(opts *Options) HandlerFunc {
	return func(ext pkix.Extension, chains [][]*x509.Certificate) error {
		if err := opts.RevDB.Put(chains[0], ext); err != nil {
			return err
		}
		return nil
	}
}

// Verify checks if the signature of the revocation was produced by the passed cert's public key.
func (r Revocation) Verify(signingCert *x509.Certificate) error {
	pubKey, ok := signingCert.PublicKey.(crypto.PublicKey)
	if !ok {
		return pkcrypto.ErrUnsupportedKey.New("%T", signingCert.PublicKey)
	}

	data := r.TBSBytes()
	if err := pkcrypto.HashAndVerifySignature(pubKey, data, r.Signature); err != nil {
		return err
	}
	return nil
}

// TBSBytes (ToBeSigned) returns the hash of the revoked certificate hash and
// the timestamp (i.e. hash(hash(cert bytes) + timestamp)).
func (r *Revocation) TBSBytes() []byte {
	var tsBytes [binary.MaxVarintLen64]byte
	binary.PutVarint(tsBytes[:], r.Timestamp)
	toHash := append(append([]byte{}, r.CertHash...), tsBytes[:]...)

	return pkcrypto.SHA256Hash(toHash)
}

// Sign generates a signature using the passed key and attaches it to the revocation.
func (r *Revocation) Sign(key crypto.PrivateKey) error {
	data := r.TBSBytes()
	sig, err := pkcrypto.HashAndSign(key, data)
	if err != nil {
		return err
	}
	r.Signature = sig
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
