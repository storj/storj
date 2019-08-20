// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package revocation

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"

	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/dbutil"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
)

var (
	mon = monkit.Package()

	// Error is a pkg/revocation error
	Error = errs.Class("revocation error")
)

// DB stores the most recently seen revocation for each nodeID
// (i.e. nodeID [CA certificate's public key hash] is the key, values is
// the most recently seen revocation).
type DB struct {
	KVStore storage.KeyValueStore
}

// NewDBFromCfg is a convenience method to create a revocation DB
// directly from a config. If the revocation extension option is not set, it
// returns a nil db with no error.
func NewDBFromCfg(cfg tlsopts.Config) (*DB, error) {
	if !cfg.Extensions.Revocation {
		return nil, nil
	}
	return NewDB(cfg.RevocationDBURL)
}

// NewDB returns a new revocation database given the URL
func NewDB(dbURL string) (*DB, error) {
	driver, source, err := dbutil.SplitConnstr(dbURL)
	if err != nil {
		return nil, extensions.ErrRevocationDB.Wrap(err)
	}

	var db *DB
	switch driver {
	case "bolt":
		db, err = newDBBolt(source)
		if err != nil {
			return nil, extensions.ErrRevocationDB.Wrap(err)
		}
	case "redis":
		db, err = newDBRedis(dbURL)
		if err != nil {
			return nil, extensions.ErrRevocationDB.Wrap(err)
		}
	default:
		return nil, extensions.ErrRevocationDB.New("database scheme not supported: %s", driver)
	}

	return db, nil
}

// newDBBolt creates a bolt-backed DB
func newDBBolt(path string) (*DB, error) {
	client, err := boltdb.New(path, extensions.RevocationBucket)
	if err != nil {
		return nil, err
	}
	return &DB{
		KVStore: client,
	}, nil
}

// newDBRedis creates a redis-backed DB.
func newDBRedis(address string) (*DB, error) {
	client, err := redis.NewClientFrom(address)
	if err != nil {
		return nil, err
	}
	return &DB{
		KVStore: client,
	}, nil
}

// Get attempts to retrieve the most recent revocation for the given cert chain
// (the  key used in the underlying database is the nodeID of the certificate chain).
func (db DB) Get(ctx context.Context, chain []*x509.Certificate) (_ *extensions.Revocation, err error) {
	defer mon.Task()(&ctx)(&err)
	nodeID, err := identity.NodeIDFromCert(chain[peertls.CAIndex])
	if err != nil {
		return nil, extensions.ErrRevocation.Wrap(err)
	}

	revBytes, err := db.KVStore.Get(ctx, nodeID.Bytes())
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
func (db DB) Put(ctx context.Context, chain []*x509.Certificate, revExt pkix.Extension) (err error) {
	defer mon.Task()(&ctx)(&err)
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

	lastRev, err := db.Get(ctx, chain)
	if err != nil {
		return err
	} else if lastRev != nil && lastRev.Timestamp >= rev.Timestamp {
		return extensions.ErrRevocationTimestamp
	}

	nodeID, err := identity.NodeIDFromCert(ca)
	if err != nil {
		return extensions.ErrRevocationDB.Wrap(err)
	}
	if err := db.KVStore.Put(ctx, nodeID.Bytes(), revExt.Value); err != nil {
		return extensions.ErrRevocationDB.Wrap(err)
	}
	return nil
}

// List lists all revocations in the store
func (db DB) List(ctx context.Context) (revs []*extensions.Revocation, err error) {
	defer mon.Task()(&ctx)(&err)
	keys, err := db.KVStore.List(ctx, []byte{}, 0)
	if err != nil {
		return nil, extensions.ErrRevocationDB.Wrap(err)
	}

	marshaledRevs, err := db.KVStore.GetAll(ctx, keys)
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
func (db DB) Close() error {
	return db.KVStore.Close()
}
