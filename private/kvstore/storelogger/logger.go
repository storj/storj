// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storelogger

import (
	"context"
	"strconv"
	"sync/atomic"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/storj/private/kvstore"
)

var mon = monkit.Package()

var id int64

// Logger implements a zap.Logger for kvstore.Store.
type Logger struct {
	log   *zap.Logger
	store kvstore.Store
}

// New creates a new Logger with log and store.
func New(log *zap.Logger, store kvstore.Store) *Logger {
	loggerid := atomic.AddInt64(&id, 1)
	name := strconv.Itoa(int(loggerid))
	return &Logger{log.Named(name), store}
}

// Put adds a value to store.
func (store *Logger) Put(ctx context.Context, key kvstore.Key, value kvstore.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	store.log.Debug("Put", zap.ByteString("key", key), zap.Int("value length", len(value)), zap.Binary("truncated value", truncate(value)))
	return store.store.Put(ctx, key, value)
}

// Get gets a value to store.
func (store *Logger) Get(ctx context.Context, key kvstore.Key) (_ kvstore.Value, err error) {
	defer mon.Task()(&ctx)(&err)
	store.log.Debug("Get", zap.ByteString("key", key))
	return store.store.Get(ctx, key)
}

// Delete deletes key and the value.
func (store *Logger) Delete(ctx context.Context, key kvstore.Key) (err error) {
	defer mon.Task()(&ctx)(&err)
	store.log.Debug("Delete", zap.ByteString("key", key))
	return store.store.Delete(ctx, key)
}

// Range iterates over all items in unspecified order.
func (store *Logger) Range(ctx context.Context, fn func(context.Context, kvstore.Key, kvstore.Value) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	store.log.Debug("Range")
	return store.store.Range(ctx, func(ctx context.Context, key kvstore.Key, value kvstore.Value) error {
		store.log.Debug("  ",
			zap.ByteString("key", key),
			zap.Int("value length", len(value)),
			zap.Binary("truncated value", truncate(value)),
		)
		return fn(ctx, key, value)
	})
}

// Close closes the store.
func (store *Logger) Close() error {
	store.log.Debug("Close")
	return store.store.Close()
}

// CompareAndSwap atomically compares and swaps oldValue with newValue.
func (store *Logger) CompareAndSwap(ctx context.Context, key kvstore.Key, oldValue, newValue kvstore.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	store.log.Debug("CompareAndSwap", zap.ByteString("key", key),
		zap.Int("old value length", len(oldValue)), zap.Int("new value length", len(newValue)),
		zap.Binary("truncated old value", truncate(oldValue)), zap.Binary("truncated new value", truncate(newValue)))
	return store.store.CompareAndSwap(ctx, key, oldValue, newValue)
}

func truncate(v kvstore.Value) (t []byte) {
	if len(v)-1 < 10 {
		t = []byte(v)
	} else {
		t = v[:10]
	}
	return t
}
