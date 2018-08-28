package storage

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type Logger struct {
	log   *zap.Logger
	store KeyValueStore
}

func NewLogger(log *zap.Logger, store KeyValueStore) *Logger {
	return &Logger{log, store}
}

func NewTestLogger(t *testing.T, store KeyValueStore) *Logger {
	return &Logger{zaptest.NewLogger(t), store}
}

// Put adds a value to store
func (store *Logger) Put(key Key, value Value) error {
	store.log.Debug("Put", zap.String("key", string(key)), zap.Binary("value", []byte(value)))
	return store.store.Put(key, value)
}

// Get gets a value to store
func (store *Logger) Get(key Key) (Value, error) {
	store.log.Debug("Get", zap.String("key", string(key)))
	return store.store.Get(key)
}

// GetAll gets all values from the store corresponding to keys
func (store *Logger) GetAll(keys Keys) (Values, error) {
	store.log.Debug("GetAll", zap.Any("keys", keys))
	return store.store.GetAll(keys)
}

// Delete deletes key and the value
func (store *Logger) Delete(key Key) error {
	store.log.Debug("Delete", zap.String("key", string(key)))
	return store.store.Delete(key)
}

// List lists all keys starting from first and upto limit items
func (store *Logger) List(first Key, limit Limit) (Keys, error) {
	store.log.Debug("List", zap.String("first", string(first)), zap.Int("limit", int(limit)))
	return store.store.List(first, limit)
}

// ListV2 lists all keys corresponding to ListOptions
func (store *Logger) ListV2(opts ListOptions) (Items, More, error) {
	store.log.Debug("ListV2", zap.Any("opts", opts))
	return store.store.ListV2(opts)
}

// ReverseList lists all keys in reverse order, starting from first
func (store *Logger) ReverseList(first Key, limit Limit) (Keys, error) {
	store.log.Debug("ReverseList", zap.String("first", string(first)), zap.Int("limit", int(limit)))
	return store.store.ReverseList(first, limit)
}

// Close closes the store
func (store *Logger) Close() error {
	store.log.Debug("Close")
	return store.store.Close()
}
