// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package statcache

import (
	"context"
	"fmt"
	"os"

	"github.com/dgraph-io/badger/v4"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/storagenode/blobstore"
)

// BadgerCache implements Cache with saving file size / mod time to badger database.
type BadgerCache struct {
	db *badger.DB
}

var _ Cache = &BadgerCache{}

// NewBadgerCache creates a new BadgerCache.
func NewBadgerCache(log *zap.Logger, dir string) (*BadgerCache, error) {
	_ = os.MkdirAll(dir, 0755)
	badgerOptions := badger.DefaultOptions(dir)
	badgerOptions.Logger = zapLogger{
		log: log,
	}
	db, err := badger.Open(badgerOptions)
	return &BadgerCache{
		db: db,
	}, err

}

// Get implements Cache.
func (b *BadgerCache) Get(ctx context.Context, namespace []byte, key []byte) (blobstore.FileInfo, bool, error) {
	txn := b.db.NewTransaction(false)
	defer txn.Discard()
	item, err := txn.Get(append(namespace, key...))
	if errs.Is(err, badger.ErrKeyNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var result blobstore.FileInfo
	err = item.Value(func(val []byte) error {
		result = deserialize(val)
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	return result, true, nil
}

// Set implements Cache.
func (b *BadgerCache) Set(ctx context.Context, namespace []byte, key []byte, value blobstore.FileInfo) error {
	txn := b.db.NewTransaction(true)
	defer txn.Discard()
	val := serialize(value)
	err := txn.Set(append(namespace, key...), val)
	if err != nil {
		return errs.Wrap(err)
	}
	return txn.Commit()
}

// Delete implements Cache.
func (b *BadgerCache) Delete(ctx context.Context, namespace []byte, key []byte) error {
	txn := b.db.NewTransaction(true)
	defer txn.Discard()
	err := txn.Delete(append(namespace, key...))
	if err != nil {
		return errs.Wrap(err)
	}
	return txn.Commit()
}

// Close implements Cache.
func (b *BadgerCache) Close() error {
	return b.db.Close()
}

type zapLogger struct {
	log *zap.Logger
}

func (z zapLogger) Errorf(s string, i ...interface{}) {
	z.log.Error(fmt.Sprintf(s, i...))
}

func (z zapLogger) Warningf(s string, i ...interface{}) {
	z.log.Warn(fmt.Sprintf(s, i...))
}

func (z zapLogger) Infof(s string, i ...interface{}) {
	/// log level is intentionally changed to debug.
	// they are not so interesting for the full storagenode process.
	z.log.Debug(fmt.Sprintf(s, i...))
}

func (z zapLogger) Debugf(s string, i ...interface{}) {
	z.log.Debug(fmt.Sprintf(s, i...))
}
