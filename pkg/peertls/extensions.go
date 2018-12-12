// Copyright (C) 2018 Storj Labs, Inc.
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

	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
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

	RevocationBucket = "revocations"
)

const (
	LeafIndex = iota
	CAIndex
)

var (
	// ExtensionIDs is a map from an enum to object identifiers used in peertls extensions.
	ExtensionIDs = map[int]asn1.ObjectIdentifier{
		SignedCertExtID: {2, 999, 1, 1},
		RevocationExtID: {2, 999, 1, 2},
	}
	// ErrExtension is used when an error occurs while processing an extensionHandler.
	ErrExtension         = errs.Class("extension error")
	ErrExtensionNotFound = errs.Class("no extension found with id:")
	ErrRevocation        = errs.Class("revocation processing error")
	ErrRevokedCert       = ErrRevocation.New("leaf certificate is revoked")
	ErrRevocationDB      = errs.Class("revocation database error")
	// ErrUniqueExtensions is used when multiple extensions have the same Id
	ErrUniqueExtensions    = ErrExtension.New("extensions are not unique")
	ErrRevocationTimestamp = ErrExtension.New("revocation timestamp is older than last known revocation")
)

// TLSExtConfig is used to bind cli flags for determining which extensions will
// be used by the server
type TLSExtConfig struct {
	Revocation          bool `help:"if true, client leafs may contain the most recent certificate revocation for the current certificate" default:"true"`
	WhitelistSignedLeaf bool `help:"if true, client leafs must contain a valid \"signed certificate extension\" (NB: verified against certs in the peer ca whitelist; i.e. if true, a whitelist must be provided)" default:"false"`
}

// ExtensionHandlers is a collection of `extensionHandler`s for convenience (see `VerifyFunc`)
type ExtensionHandlers []extensionHandler

type extensionVerificationFunc func(pkix.Extension, [][]*x509.Certificate) error

type extensionHandler struct {
	id asn1.ObjectIdentifier
	f  extensionVerificationFunc
}

type ParseExtOptions struct {
	CAWhitelist []*x509.Certificate
	RevDB       *RevocationDB
}

type Revocation struct {
	Timestamp int64
	CertHash  []byte
	Signature []byte
}

type RevocationDB struct {
	DB storage.KeyValueStore
}

// ParseExtensions parses an extension config into a slice of extension handlers
// with their respective ids (`asn1.ObjectIdentifier`) and a function (`f`) which
// can be used in the context of peer certificate verification.
func ParseExtensions(c TLSExtConfig, opts ParseExtOptions) (exts ExtensionHandlers) {
	if c.WhitelistSignedLeaf {
		exts = append(exts, extensionHandler{
			id: ExtensionIDs[SignedCertExtID],
			f:  verifyCAWhitelistSignedLeafFunc(opts.CAWhitelist),
		})
	}

	if c.Revocation {
		exts = append(exts, extensionHandler{
			id: ExtensionIDs[RevocationExtID],
			f: func(certExt pkix.Extension, chains [][]*x509.Certificate) error {
				if err := opts.RevDB.Put(chains[0], certExt); err != nil {
					return err
				}
				return nil
			},
		})
	}

	return exts
}

func NewRevocationDBBolt(path string) (*RevocationDB, error) {
	client, err := boltdb.New(path, RevocationBucket)
	if err != nil {
		return nil, err
	}
	return &RevocationDB{
		DB: client,
	}, nil
}

func NewRevocationDBRedis(address string) (*RevocationDB, error) {
	client, err := redis.NewClientFrom(address)
	if err != nil {
		return nil, err
	}
	return &RevocationDB{
		DB: client,
	}, nil
}

func NewRevocationExt(key crypto.PrivateKey, revokedCert *x509.Certificate) (pkix.Extension, error) {
	nowUnix := time.Now().Unix()

	hash, err := hashBytes(revokedCert.Raw)
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

// AddSignedCertExt adds a "signed certificate extensionHandler" to the passed cert,
// using the passed private key.
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

func AddExtension(cert *x509.Certificate, exts ...pkix.Extension) (err error) {
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
// `extensionHandler#f` when it finds a match by id (`asn1.ObjectIdentifier`)
func (e ExtensionHandlers) VerifyFunc() PeerCertVerificationFunc {
	return func(_ [][]byte, parsedChains [][]*x509.Certificate) error {
		for _, ext := range parsedChains[0][LeafIndex].Extensions {
			for _, v := range e {
				if v.id.Equal(ext.Id) {
					err := v.f(ext, parsedChains)
					if err != nil {
						return ErrExtension.Wrap(err)
					}
				}
			}
		}
		return nil
	}
}

func (r RevocationDB) Get(chain []*x509.Certificate) (*Revocation, error) {
	hash, err := hashBytes(chain[CAIndex].Raw)
	if err != nil {
		return nil, ErrRevocation.Wrap(err)
	}

	revBytes, err := r.DB.Get(hash)
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return nil, ErrRevocationDB.Wrap(err)
	}
	if revBytes == nil {
		return nil, nil
	}

	rev := new(Revocation)
	if err = rev.Unmarshal(revBytes); err != nil {
		return rev, ErrRevocationDB.Wrap(err)
	}
	return rev, nil
}

func (r RevocationDB) Put(chain []*x509.Certificate, revExt pkix.Extension) error {
	ca := chain[CAIndex]
	var rev Revocation
	if err := rev.Unmarshal(revExt.Value); err != nil {
		return err
	}

	// TODO: what happens if cert/timestamp/sig is empty/garbage?

	if err := rev.Verify(ca); err != nil {
		return err
	}

	lastRev, err := r.Get(chain)
	if err != nil {
		return err
	} else if lastRev != nil && lastRev.Timestamp >= rev.Timestamp {
		return ErrRevocationTimestamp
	}

	hash, err := hashBytes(ca.Raw)
	if err := r.DB.Put(hash, revExt.Value); err != nil {
		return err
	}
	return nil
}

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

func (r *Revocation) TBSBytes() ([]byte, error) {

	toHash := new(bytes.Buffer)
	_, err := toHash.Write(r.CertHash)
	if err != nil {
		return nil, ErrExtension.Wrap(err)
	}

	// NB: append timestamp to revoked cert bytes
	binary.PutVarint(toHash.Bytes(), r.Timestamp)

	return hashBytes(toHash.Bytes())
}

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

		leaf := chains[0][0]
		for _, ca := range caWhitelist {
			err := VerifySignature(certExt.Value, leaf.RawTBSCertificate, ca.PublicKey)
			if err == nil {
				return nil
			}
		}
		return ErrVerifyCAWhitelist.New("leaf extension")
	}
}
