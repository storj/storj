package storelogger

import (
	"testing"

	"storj.io/storj/storage"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type Logger struct {
	log   *zap.Logger
	store storage.KeyValueStore
}

func New(log *zap.Logger, store storage.KeyValueStore) *Logger {
	return &Logger{log, store}
}

func NewTest(t *testing.T, store storage.KeyValueStore) *Logger {
	return New(zaptest.NewLogger(t), store)
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
func (store *Logger) List(first storage.Key, limit storage.Limit) (storage.Keys, error) {
	store.log.Debug("List", zap.String("first", string(first)), zap.Int("limit", int(limit)))
	return store.store.List(first, limit)
}

// Allows to iterate over collapsed items
// with prefix starting from first or the nearest next key
func (store *Logger) Iterate(prefix, first storage.Key, delimiter byte, fn func(storage.Iterator) error) error {
	store.log.Debug("Iterate", zap.String("prefix", string(first)), zap.String("first", string(first)))
	return store.store.Iterate(prefix, first, delimiter, func(it storage.Iterator) error {
		return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
			ok := it.Next(item)
			if ok {
				store.log.Debug("  ", zap.String("key", string(item.Key)), zap.Binary("value", item.Value))
			}
			return ok
		}))
	})
}

func (store *Logger) IterateAll(prefix, first storage.Key, fn func(storage.Iterator) error) error {
	store.log.Debug("IterateAll", zap.String("prefix", string(first)), zap.String("first", string(first)))
	return store.store.IterateAll(prefix, first, func(it storage.Iterator) error {
		return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
			ok := it.Next(item)
			if ok {
				store.log.Debug("  ", zap.String("key", string(item.Key)), zap.Binary("value", item.Value))
			}
			return ok
		}))
	})
}

// ReverseList lists all keys in reverse order, starting from first
func (store *Logger) ReverseList(first storage.Key, limit storage.Limit) (storage.Keys, error) {
	store.log.Debug("ReverseList", zap.String("first", string(first)), zap.Int("limit", int(limit)))
	return store.store.ReverseList(first, limit)
}

// Close closes the store
func (store *Logger) Close() error {
	store.log.Debug("Close")
	return store.store.Close()
}
