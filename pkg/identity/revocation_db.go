// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"storj.io/storj/internal/dbutil"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
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

// NewRevocationDB returns a new revocation database given the URL
func NewRevocationDB(revocationDBURL string) (*RevocationDB, error) {
	driver, source, err := dbutil.SplitConnstr(revocationDBURL)
	if err != nil {
		return nil, extensions.ErrRevocationDB.Wrap(err)
	}

	var db *RevocationDB
	switch driver {
	case "bolt":
		db, err = newRevocationDBBolt(source)
		if err != nil {
			return nil, extensions.ErrRevocationDB.Wrap(err)
		}
	case "redis":
		db, err = newRevocationDBRedis(revocationDBURL)
		if err != nil {
			return nil, extensions.ErrRevocationDB.Wrap(err)
		}
	default:
		return nil, extensions.ErrRevocationDB.New("database scheme not supported: %s", driver)
	}

	return db, nil
}

// newRevocationDBBolt creates a bolt-backed RevocationDB
func newRevocationDBBolt(path string) (*RevocationDB, error) {
	client, err := boltdb.New(path, extensions.RevocationBucket)
	if err != nil {
		return nil, err
	}
	return &RevocationDB{
		DB: client,
	}, nil
}

// newRevocationDBRedis creates a redis-backed RevocationDB.
func newRevocationDBRedis(address string) (*RevocationDB, error) {
	client, err := redis.NewClientFrom(address)
	if err != nil {
		return nil, err
	}
	return &RevocationDB{
		DB: client,
	}, nil
}

// Get attempts to retrieve the most recent revocation for the given cert chain
// (the  key used in the underlying database is the nodeID of the certificate chain).
func (r RevocationDB) Get(chain []*x509.Certificate) (*extensions.Revocation, error) {
	nodeID, err := NodeIDFromCert(chain[peertls.CAIndex])
	if err != nil {
		return nil, extensions.ErrRevocation.Wrap(err)
	}

	revBytes, err := r.DB.Get(nodeID.Bytes())
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return nil, extensions.ErrRevocationDB.Wrap(err)
	}
	if revBytes == nil {
		return nil, nil
	}

	rev := new(extensions.Revocation)
	if err = rev.Unmarshal(revBytes); err != nil {
		return rev, extensions.ErrRevocationDB.Wrap(err)
	}
	return rev, nil
}

// Put stores the most recent revocation for the given cert chain IF the timestamp
// is newer than the current value (the  key used in the underlying database is
// the nodeID of the certificate chain).
func (r RevocationDB) Put(chain []*x509.Certificate, revExt pkix.Extension) error {
	ca := chain[peertls.CAIndex]
	var rev extensions.Revocation
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
		return extensions.ErrRevocationTimestamp
	}

	nodeID, err := NodeIDFromCert(ca)
	if err != nil {
		return extensions.ErrRevocationDB.Wrap(err)
	}
	if err := r.DB.Put(nodeID.Bytes(), revExt.Value); err != nil {
		return extensions.ErrRevocationDB.Wrap(err)
	}
	return nil
}

// List lists all revocations in the store
func (r RevocationDB) List() (revs []*extensions.Revocation, err error) {
	keys, err := r.DB.List([]byte{}, 0)
	if err != nil {
		return nil, extensions.ErrRevocationDB.Wrap(err)
	}

	marshaledRevs, err := r.DB.GetAll(keys)
	if err != nil {
		return nil, extensions.ErrRevocationDB.Wrap(err)
	}

	for _, revBytes := range marshaledRevs {
		rev := new(extensions.Revocation)
		if err := rev.Unmarshal(revBytes); err != nil {
			return nil, extensions.ErrRevocationDB.Wrap(err)
		}

		revs = append(revs, rev)
	}
	return revs, nil
}

// Close closes the underlying store
func (r RevocationDB) Close() error {
	return r.DB.Close()
}
