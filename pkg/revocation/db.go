// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package revocation

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"

	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/identity"
	"storj.io/common/peertls"
	"storj.io/common/peertls/extensions"
	"storj.io/storj/storage"
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
	store storage.KeyValueStore
}

// Get attempts to retrieve the most recent revocation for the given cert chain
// (the  key used in the underlying database is the nodeID of the certificate chain).
func (db *DB) Get(ctx context.Context, chain []*x509.Certificate) (_ *extensions.Revocation, err error) {
	defer mon.Task()(&ctx)(&err)

	if db.store == nil {
		return nil, nil
	}

	nodeID, err := identity.NodeIDFromCert(chain[peertls.CAIndex])
	if err != nil {
		return nil, extensions.ErrRevocation.Wrap(err)
	}

	revBytes, err := db.store.Get(ctx, nodeID.Bytes())
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
func (db *DB) Put(ctx context.Context, chain []*x509.Certificate, revExt pkix.Extension) (err error) {
	defer mon.Task()(&ctx)(&err)

	if db.store == nil {
		return extensions.ErrRevocationDB.New("not supported")
	}

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
	if err := db.store.Put(ctx, nodeID.Bytes(), revExt.Value); err != nil {
		return extensions.ErrRevocationDB.Wrap(err)
	}
	return nil
}

// List lists all revocations in the store
func (db *DB) List(ctx context.Context) (revs []*extensions.Revocation, err error) {
	defer mon.Task()(&ctx)(&err)

	if db.store == nil {
		return nil, nil
	}

	keys, err := db.store.List(ctx, []byte{}, 0)
	if err != nil {
		return nil, extensions.ErrRevocationDB.Wrap(err)
	}

	marshaledRevs, err := db.store.GetAll(ctx, keys)
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

// TestGetStore returns the internal store for testing.
func (db *DB) TestGetStore() storage.KeyValueStore {
	return db.store
}

// Close closes the underlying store
func (db *DB) Close() error {
	if db.store == nil {
		return nil
	}
	return db.store.Close()
}
