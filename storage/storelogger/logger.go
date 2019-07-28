// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storelogger

import (
	"context"
	"strconv"
	"sync/atomic"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/storage"
)

var mon = monkit.Package()

var id int64

// Logger implements a zap.Logger for storage.KeyValueStore
type Logger struct {
	log   *zap.Logger
	store storage.KeyValueStore
}

// New creates a new Logger with log and store
func New(log *zap.Logger, store storage.KeyValueStore) *Logger {
	loggerid := atomic.AddInt64(&id, 1)
	name := strconv.Itoa(int(loggerid))
	return &Logger{log.Named(name), store}
}

// Put adds a value to store
func (store *Logger) Put(ctx context.Context, key storage.Key, value storage.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	store.log.Debug("Put", zap.ByteString("key", key), zap.Int("value length", len(value)), zap.Binary("truncated value", truncate(value)))
	return store.store.Put(ctx, key, value)
}

// Get gets a value to store
func (store *Logger) Get(ctx context.Context, key storage.Key) (_ storage.Value, err error) {
	defer mon.Task()(&ctx)(&err)
	store.log.Debug("Get", zap.ByteString("key", key))
	return store.store.Get(ctx, key)
}

// GetAll gets all values from the store corresponding to keys
func (store *Logger) GetAll(ctx context.Context, keys storage.Keys) (_ storage.Values, err error) {
	defer mon.Task()(&ctx)(&err)
	store.log.Debug("GetAll", zap.Any("keys", keys))
	return store.store.GetAll(ctx, keys)
}

// Delete deletes key and the value
func (store *Logger) Delete(ctx context.Context, key storage.Key) (err error) {
	defer mon.Task()(&ctx)(&err)
	store.log.Debug("Delete", zap.ByteString("key", key))
	return store.store.Delete(ctx, key)
}

// List lists all keys starting from first and upto limit items
func (store *Logger) List(ctx context.Context, first storage.Key, limit int) (_ storage.Keys, err error) {
	defer mon.Task()(&ctx)(&err)
	keys, err := store.store.List(ctx, first, limit)
	store.log.Debug("List", zap.ByteString("first", first), zap.Int("limit", limit), zap.Strings("keys", keys.Strings()))
	return keys, err
}

// Iterate iterates over items based on opts
func (store *Logger) Iterate(ctx context.Context, opts storage.IterateOptions, fn func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	store.log.Debug("Iterate",
		zap.ByteString("prefix", opts.Prefix),
		zap.ByteString("first", opts.First),
		zap.Bool("recurse", opts.Recurse),
		zap.Bool("reverse", opts.Reverse),
	)
	return store.store.Iterate(ctx, opts, func(ctx context.Context, it storage.Iterator) error {
		return fn(ctx, storage.IteratorFunc(func(ctx context.Context, item *storage.ListItem) bool {
			ok := it.Next(ctx, item)
			if ok {
				store.log.Debug("  ",
					zap.ByteString("key", item.Key),
					zap.Int("value length", len(item.Value)),
					zap.Binary("truncated value", truncate(item.Value)),
				)
			}
			return ok
		}))
	})
}

// Close closes the store
func (store *Logger) Close() error {
	store.log.Debug("Close")
	return store.store.Close()
}

// CompareAndSwap atomically compares and swaps oldValue with newValue
func (store *Logger) CompareAndSwap(ctx context.Context, key storage.Key, oldValue, newValue storage.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	store.log.Debug("CompareAndSwap", zap.ByteString("key", key),
		zap.Int("old value length", len(oldValue)), zap.Int("new value length", len(newValue)),
		zap.Binary("truncated old value", truncate(oldValue)), zap.Binary("truncated new value", truncate(newValue)))
	return store.store.CompareAndSwap(ctx, key, oldValue, newValue)
}

func truncate(v storage.Value) (t []byte) {
	if len(v)-1 < 10 {
		t = []byte(v)
	} else {
		t = v[:10]
	}
	return t
}
