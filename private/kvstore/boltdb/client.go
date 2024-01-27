// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"bytes"
	"context"
	"sync/atomic"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.etcd.io/bbolt"

	"storj.io/storj/private/kvstore"
)

var mon = monkit.Package()

// Error is the default boltdb errs class.
var Error = errs.Class("boltdb")

// Client is the entrypoint into a bolt data store.
type Client struct {
	db     *bbolt.DB
	Path   string
	Bucket []byte

	referenceCount *int32
}

const (
	// fileMode sets permissions so owner can read and write.
	fileMode       = 0600
	defaultTimeout = 1 * time.Second
)

// New instantiates a new BoltDB client given db file path, and a bucket name.
func New(path, bucket string) (*Client, error) {
	db, err := bbolt.Open(path, fileMode, &bbolt.Options{Timeout: defaultTimeout})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = Error.Wrap(db.Update(func(tx *bbolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(bucket))
		return err
	}))
	if err != nil {
		if closeErr := Error.Wrap(db.Close()); closeErr != nil {
			return nil, errs.Combine(err, closeErr)
		}
		return nil, err
	}

	refCount := new(int32)
	*refCount = 1

	return &Client{
		db:             db,
		referenceCount: refCount,
		Path:           path,
		Bucket:         []byte(bucket),
	}, nil
}

func (client *Client) update(fn func(*bbolt.Bucket) error) error {
	return Error.Wrap(client.db.Update(func(tx *bbolt.Tx) error {
		return fn(tx.Bucket(client.Bucket))
	}))
}

func (client *Client) batch(fn func(*bbolt.Bucket) error) error {
	return Error.Wrap(client.db.Batch(func(tx *bbolt.Tx) error {
		return fn(tx.Bucket(client.Bucket))
	}))
}

func (client *Client) view(fn func(*bbolt.Bucket) error) error {
	return Error.Wrap(client.db.View(func(tx *bbolt.Tx) error {
		return fn(tx.Bucket(client.Bucket))
	}))
}

// Put adds a key/value to boltDB in a batch, where boltDB commits the batch to disk every
// 1000 operations or 10ms, whichever is first. The MaxBatchDelay are using default settings.
// Ref: https://github.com/boltdb/bolt/blob/master/db.go#L160
// Note: when using this method, check if it needs to be executed asynchronously
// since it blocks for the duration db.MaxBatchDelay.
func (client *Client) Put(ctx context.Context, key kvstore.Key, value kvstore.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	start := time.Now()
	if key.IsZero() {
		return kvstore.ErrEmptyKey.New("")
	}

	err = client.batch(func(bucket *bbolt.Bucket) error {
		return bucket.Put(key, value)
	})
	mon.IntVal("boltdb_batch_time_elapsed").Observe(int64(time.Since(start)))
	return err
}

// PutAndCommit adds a key/value to BoltDB and writes it to disk.
func (client *Client) PutAndCommit(ctx context.Context, key kvstore.Key, value kvstore.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return kvstore.ErrEmptyKey.New("")
	}

	return client.update(func(bucket *bbolt.Bucket) error {
		return bucket.Put(key, value)
	})
}

// Get looks up the provided key from boltdb returning either an error or the result.
func (client *Client) Get(ctx context.Context, key kvstore.Key) (_ kvstore.Value, err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return nil, kvstore.ErrEmptyKey.New("")
	}

	var value kvstore.Value
	err = client.view(func(bucket *bbolt.Bucket) error {
		data := bucket.Get([]byte(key))
		if len(data) == 0 {
			return kvstore.ErrKeyNotFound.New("%q", key)
		}
		value = kvstore.CloneValue(kvstore.Value(data))
		return nil
	})
	return value, err
}

// Delete deletes a key/value pair from boltdb, for a given the key.
func (client *Client) Delete(ctx context.Context, key kvstore.Key) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return kvstore.ErrEmptyKey.New("")
	}

	return client.update(func(bucket *bbolt.Bucket) error {
		return bucket.Delete(key)
	})
}

// Close closes a BoltDB client.
func (client *Client) Close() (err error) {
	if atomic.AddInt32(client.referenceCount, -1) == 0 {
		return Error.Wrap(client.db.Close())
	}
	return nil
}

// Range iterates over all items in unspecified order.
func (client *Client) Range(ctx context.Context, fn func(context.Context, kvstore.Key, kvstore.Value) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	return client.view(func(bucket *bbolt.Bucket) error {
		return bucket.ForEach(func(k, v []byte) error {
			return fn(ctx, kvstore.Key(k), kvstore.Value(v))
		})
	})
}

// CompareAndSwap atomically compares and swaps oldValue with newValue.
func (client *Client) CompareAndSwap(ctx context.Context, key kvstore.Key, oldValue, newValue kvstore.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return kvstore.ErrEmptyKey.New("")
	}

	return client.update(func(bucket *bbolt.Bucket) error {
		data := bucket.Get([]byte(key))
		if len(data) == 0 {
			if oldValue != nil {
				return kvstore.ErrKeyNotFound.New("%q", key)
			}

			if newValue == nil {
				return nil
			}

			return Error.Wrap(bucket.Put(key, newValue))
		}

		if !bytes.Equal(kvstore.Value(data), oldValue) {
			return kvstore.ErrValueChanged.New("%q", key)
		}

		if newValue == nil {
			return Error.Wrap(bucket.Delete(key))
		}

		return Error.Wrap(bucket.Put(key, newValue))
	})
}
