// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"bytes"
	"crypto/x509"
	"crypto/x509/pkix"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
)

// RevocationDB stores the most recently seen revocation for each nodeID
// (i.e. nodeID [CA certificate's public key hash] is the key, values is
// the most recently seen revocation).
type RevocationDB struct {
	DB storage.KeyValueStore
}

// Get attempts to retrieve the most recent revocation for the given cert chain
// (the  key used in the underlying database is the nodeID of the certificate chain).
func (r RevocationDB) Get(chain []*x509.Certificate) (*peertls.Revocation, error) {
	nodeID, err := NodeIDFromKey(chain[peertls.CAIndex].PublicKey)
	if err != nil {
		return nil, peertls.ErrRevocation.Wrap(err)
	}

	revBytes, err := r.DB.Get(nodeID.Bytes())
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return nil, peertls.ErrRevocationDB.Wrap(err)
	}
	if revBytes == nil {
		return nil, nil
	}

	rev := new(peertls.Revocation)
	if err = rev.Unmarshal(revBytes); err != nil {
		return rev, peertls.ErrRevocationDB.Wrap(err)
	}
	return rev, nil
}

// Put stores the most recent revocation for the given cert chain IF the timestamp
// is newer than the current value (the  key used in the underlying database is
// the nodeID of the certificate chain).
func (r RevocationDB) Put(chain []*x509.Certificate, revExt pkix.Extension) error {
	ca := chain[peertls.CAIndex]
	var rev peertls.Revocation
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
		return peertls.ErrRevocationTimestamp
	}

	nodeID, err := NodeIDFromKey(ca.PublicKey)
	if err != nil {
		return peertls.ErrRevocationDB.Wrap(err)
	}
	if err := r.DB.Put(nodeID.Bytes(), revExt.Value); err != nil {
		return peertls.ErrRevocationDB.Wrap(err)
	}
	return nil
}

// List lists all revocations in the store
func (r RevocationDB) List() (revs []*peertls.Revocation, err error) {
	keys, err := r.DB.List([]byte{}, 0)
	if err != nil {
		return nil, peertls.ErrRevocationDB.Wrap(err)
	}

	marshaledRevs, err := r.DB.GetAll(keys)
	if err != nil {
		return nil, peertls.ErrRevocationDB.Wrap(err)
	}

	for _, revBytes := range marshaledRevs {
		rev := new(peertls.Revocation)
		if err := rev.Unmarshal(revBytes); err != nil {
			return nil, peertls.ErrRevocationDB.Wrap(err)
		}

		revs = append(revs, rev)
	}
	return revs, nil
}

// Close closes the underlying store
func (r RevocationDB) Close() error {
	return r.DB.Close()
}

// NewRevDB returns a new revocation database given the URL
func NewRevDB(revocationDBURL string) (*RevocationDB, error) {
	driver, source, err := utils.SplitDBURL(revocationDBURL)
	if err != nil {
		return nil, peertls.ErrRevocationDB.Wrap(err)
	}

	var db *RevocationDB
	switch driver {
	case "bolt":
		db, err = NewRevocationDBBolt(source)
		if err != nil {
			return nil, peertls.ErrRevocationDB.Wrap(err)
		}
	case "redis":
		db, err = NewRevocationDBRedis(revocationDBURL)
		if err != nil {
			return nil, peertls.ErrRevocationDB.Wrap(err)
		}
	default:
		return nil, peertls.ErrRevocationDB.New("database scheme not supported: %s", driver)
	}

	return db, nil
}

// NewRevocationDBBolt creates a bolt-backed RevocationDB
func NewRevocationDBBolt(path string) (*RevocationDB, error) {
	client, err := boltdb.New(path, peertls.RevocationBucket)
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

// VerifyUnrevokedChainFunc returns a peer certificate verification function which
// returns an error if the incoming cert chain contains a revoked CA or leaf.
func VerifyUnrevokedChainFunc(revDB *RevocationDB) peertls.PeerCertVerificationFunc {
	return func(_ [][]byte, chains [][]*x509.Certificate) error {
		leaf := chains[0][peertls.LeafIndex]
		ca := chains[0][peertls.CAIndex]
		lastRev, lastRevErr := revDB.Get(chains[0])
		if lastRevErr != nil {
			return peertls.ErrExtension.Wrap(lastRevErr)
		}
		if lastRev == nil {
			return nil
		}

		// NB: we trust that anything that made it into the revocation DB is valid
		//		(i.e. no need for further verification)
		switch {
		case bytes.Equal(lastRev.CertHash, ca.Raw):
			fallthrough
		case bytes.Equal(lastRev.CertHash, leaf.Raw):
			return peertls.ErrRevokedCert
		default:
			return nil
		}
	}
}
