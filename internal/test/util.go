// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"

	"storj.io/storj/storage"
)

// KvStore is an in-memory, crappy key/value store type for testing
type KvStore map[string]storage.Value

// MockKeyValueStore is a `KeyValueStore` type used for testing (see storj.io/storj/storage/common.go)
type MockKeyValueStore struct {
	Data         KvStore
	GetCalled    int
	PutCalled    int
	ListCalled   int
	DeleteCalled int
	CloseCalled  int
	PingCalled   int
}

// RedisDone is a function type that describes the callback returned by `EnsureRedis`
type RedisDone func()

// RedisServer is a struct which holds and manages the state of a `redis-server` process
type RedisServer struct {
	cmd     *exec.Cmd
	started bool
}

var (
	// ErrMissingKey is the error class returned if a key is not in the mock store
	ErrMissingKey = errs.Class("missing")

	// ErrForced is the error class returned when the forced error flag is passed
	// to mock an error
	ErrForced = errs.Class("error forced by using 'error' key in mock")

	redisRefs = map[string]bool{}
	testRedis = &RedisServer{
		started: false,
	}
)

// Get looks up the provided key from the MockKeyValueStore returning either an error or the result.
func (m *MockKeyValueStore) Get(key storage.Key) (storage.Value, error) {
	m.GetCalled++
	if key.String() == "error" {
		return storage.Value{}, ErrForced.New("forced error")
	}
	v, ok := m.Data[key.String()]
	if !ok {
		return storage.Value{}, ErrMissingKey.New("key %v missing", key)
	}

	return v, nil
}

// Put adds a value to the provided key in the MockKeyValueStore, returning an error on failure.
func (m *MockKeyValueStore) Put(key storage.Key, value storage.Value) error {
	m.PutCalled++
	m.Data[key.String()] = value
	return nil
}

// Delete deletes a key/value pair from the MockKeyValueStore, for a given the key
func (m *MockKeyValueStore) Delete(key storage.Key) error {
	m.DeleteCalled++
	delete(m.Data, key.String())
	return nil
}

// List returns either a list of keys for which the MockKeyValueStore has values or an error.
func (m *MockKeyValueStore) List() (_ storage.Keys, _ error) {
	m.ListCalled++
	keys := storage.Keys{}
	for k := range m.Data {
		keys = append(keys, storage.Key(k))
	}

	return keys, nil
}

// Close closes the client
func (m *MockKeyValueStore) Close() error {
	m.CloseCalled++
	return nil
}

// Ping is called by some redis client code
func (m *MockKeyValueStore) Ping() error {
	m.PingCalled++
	return nil
}

// NewMockKeyValueStore returns a mocked `KeyValueStore` implementation for testing
func NewMockKeyValueStore(d KvStore) *MockKeyValueStore {
	return &MockKeyValueStore{
		Data:         d,
		GetCalled:    0,
		PutCalled:    0,
		ListCalled:   0,
		DeleteCalled: 0,
		CloseCalled:  0,
		PingCalled:   0,
	}
}

// EnsureRedis attempts to start the `redis-server` binary
func EnsureRedis(t *testing.T) (_ RedisDone) {
	flag.Set("redisAddress", "127.0.0.1:6379")

	index, _ := randomHex(5)
	redisRefs[index] = true

	if testRedis.started != true {
		testRedis.start(t)
	}

	return func() {
		if v := recover(); v != nil {
			testRedis.stop()
			panic(v)
		}

		redisRefs[index] = false

		if !(redisRefCount() > 0) {
			testRedis.stop()
		}
	}
}

func redisRefCount() (_ int) {
	count := 0
	for _, ref := range redisRefs {
		if ref {
			count++
		}
	}

	return count
}

func (r *RedisServer) start(t *testing.T) {
	r.cmd = &exec.Cmd{}
	cmd := r.cmd

	logPath, err := filepath.Abs("test_redis-server.log")
	assert.NoError(t, err)

	binPath, err := exec.LookPath("redis-server")
	assert.NoError(t, err)

	log, err := os.Create(logPath)
	assert.NoError(t, err)

	cmd.Path = binPath
	cmd.Stdout = log

	go func() {
		r.started = true

		if err := cmd.Run(); err != nil {
			// TODO(bryanchriswhite) error checking
		}
	}()

	time.Sleep(2 * time.Second)
}

func (r *RedisServer) stop() {
	r.started = false
	r.cmd.Process.Kill()
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
