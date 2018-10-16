// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storelogger

import (
	"strconv"
	"sync/atomic"

	"go.uber.org/zap"

	"storj.io/storj/storage"
)

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
func (store *Logger) Put(key storage.Key, value storage.Value) error {
	store.log.Debug("Put", zap.String("key", string(key)), zap.Binary("value", []byte(value)))
	return store.store.Put(key, value)
}

// Get gets a value to store
func (store *Logger) Get(key storage.Key) (storage.Value, error) {
	store.log.Debug("Get", zap.String("key", string(key)))
	return store.store.Get(key)
}

// GetAll gets all values from the store corresponding to keys
func (store *Logger) GetAll(keys storage.Keys) (storage.Values, error) {
	store.log.Debug("GetAll", zap.Any("keys", keys))
	return store.store.GetAll(keys)
}

// Delete deletes key and the value
func (store *Logger) Delete(key storage.Key) error {
	store.log.Debug("Delete", zap.String("key", string(key)))
	return store.store.Delete(key)
}

// List lists all keys starting from first and upto limit items
func (store *Logger) List(first storage.Key, limit int) (storage.Keys, error) {
	keys, err := store.store.List(first, limit)
	store.log.Debug("List", zap.String("first", string(first)), zap.Int("limit", limit), zap.Any("keys", keys.Strings()))
	return keys, err
}

// ReverseList lists all keys in reverse order, starting from first
func (store *Logger) ReverseList(first storage.Key, limit int) (storage.Keys, error) {
	keys, err := store.store.ReverseList(first, limit)
	store.log.Debug("ReverseList", zap.String("first", string(first)), zap.Int("limit", limit), zap.Any("keys", keys.Strings()))
	return keys, err
}

// Iterate iterates over items based on opts
func (store *Logger) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	store.log.Debug("Iterate",
		zap.String("prefix", string(opts.Prefix)),
		zap.String("first", string(opts.First)),
		zap.Bool("recurse", opts.Recurse),
		zap.Bool("reverse", opts.Reverse),
	)
	return store.store.Iterate(opts, func(it storage.Iterator) error {
		return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
			ok := it.Next(item)
			if ok {
				store.log.Debug("  ", zap.String("key", string(item.Key)), zap.Binary("value", item.Value))
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
