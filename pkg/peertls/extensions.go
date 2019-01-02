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
	"go.uber.org/zap"

	"storj.io/storj/pkg/utils"
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
	// ErrRevokedCert is used when a certificate is revoked and not expected to be
	ErrRevokedCert = ErrRevocation.New("leaf certificate is revoked")
	// ErrUniqueExtensions is used when multiple extensions have the same Id
	ErrUniqueExtensions = ErrExtension.New("extensions are not unique")
	// ErrRevocationTimestamp is used when a revocation's timestamp is older than the last recorded revocation
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
	id     asn1.ObjectIdentifier
	verify extensionVerificationFunc
}

// ParseExtOptions holds options for calling `ParseExtensions`
type ParseExtOptions struct {
	CAWhitelist []*x509.Certificate
	RevDB       *RevocationDB
}

// Revocation represents a certificate revocation for storage in the revocation
// database and for use in a TLS extension
type Revocation struct {
	Timestamp int64
	CertHash  []byte
	Signature []byte
}

// RevocationDB stores the most recently seen revocation for each nodeID
// (i.e. nodeID [CA certificate hash] is the key, value is the most
// recently seen revocation).
type RevocationDB struct {
	DB storage.KeyValueStore
}

// ParseExtensions parses an extension config into a slice of extension handlers
// with their respective ids (`asn1.ObjectIdentifier`) and a "verify" function
// to be used in the context of peer certificate verification.
func ParseExtensions(c TLSExtConfig, opts ParseExtOptions) (handlers ExtensionHandlers) {
	if c.WhitelistSignedLeaf {
		handlers = append(handlers, extensionHandler{
			id:     ExtensionIDs[SignedCertExtID],
			verify: verifyCAWhitelistSignedLeafFunc(opts.CAWhitelist),
		})
	}

	if c.Revocation {
		handlers = append(handlers, extensionHandler{
			id: ExtensionIDs[RevocationExtID],
			verify: func(certExt pkix.Extension, chains [][]*x509.Certificate) error {
				if err := opts.RevDB.Put(chains[0], certExt); err != nil {
					return err
				}
				return nil
			},
		})
	}

	return handlers
}

// NewRevocationDBBolt creates a bolt-backed RevocationDB
func NewRevocationDBBolt(path string) (*RevocationDB, error) {
	client, err := boltdb.New(path, RevocationBucket)
	if err != nil {
		return nil, err
	}
	return &RevocationDB{
		DB: client,
	}, nil
}

// NewRevocationDBRedis creates a redis-backed RevocationDB.
func NewRevocationDBRedis(address string) (*RevocationDB, error) {
	client, err := redis.NewClientFrom(address)
	if err != nil {
		return nil, err
	}
	return &RevocationDB{
		DB: client,
	}, nil
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
			if ext, ok := leafExts[handler.id.String()]; ok {
				err := handler.verify(ext, parsedChains)
				if err != nil {
					return ErrExtension.Wrap(err)
				}
			}
		}
		return nil
	}
}

// Get attempts to retrieve the most recent revocation for the given cert chain
// (the  key used in the underlying database is the hash of the CA cert bytes).
func (r RevocationDB) Get(chain []*x509.Certificate) (*Revocation, error) {
	hash, err := SHA256Hash(chain[CAIndex].Raw)
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

// Put stores the most recent revocation for the given cert chain IF the timestamp
// is newer than the current value (the  key used in the underlying database is
// the hash of the CA cert bytes).
func (r RevocationDB) Put(chain []*x509.Certificate, revExt pkix.Extension) error {
	ca := chain[CAIndex]
	var rev Revocation
	if err := rev.Unmarshal(revExt.Value); err != nil {
		return err
	}

	// TODO: do we care if cert/timestamp/sig is empty/garbage?
	// TODO(bryanchriswhite): test empty/garbage cert/timestamp/sig

	if err := rev.Verify(ca); err != nil {
		return err
	}

	lastRev, err := r.Get(chain)
	if err != nil {
		return err
	} else if lastRev != nil && lastRev.Timestamp >= rev.Timestamp {
		return ErrRevocationTimestamp
	}

	hash, err := SHA256Hash(ca.Raw)
	if err != nil {
		return err
	}
	if err := r.DB.Put(hash, revExt.Value); err != nil {
		return err
	}
	return nil
}

// Close closes the underlying store
func (r RevocationDB) Close() error {
	return r.DB.Close()
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

// NewRevDB returns a new revocation database given the URL
func NewRevDB(revocationDBURL string) (*RevocationDB, error) {
	driver, source, err := utils.SplitDBURL(revocationDBURL)
	if err != nil {
		return nil, ErrRevocationDB.Wrap(err)
	}

	var db *RevocationDB
	switch driver {
	case "bolt":
		db, err = NewRevocationDBBolt(source)
		if err != nil {
			return nil, ErrRevocationDB.Wrap(err)
		}
		zap.S().Info("Starting overlay cache with BoltDB")
	case "redis":
		db, err = NewRevocationDBRedis(revocationDBURL)
		if err != nil {
			return nil, ErrRevocationDB.Wrap(err)
		}
		zap.S().Info("Starting overlay cache with Redis")
	default:
		return nil, ErrRevocationDB.New("database scheme not supported: %s", driver)
	}

	return db, nil
}
