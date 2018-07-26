// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"

	"storj.io/storj/storage"
)

// KvStore is an in-memory, crappy key/value store type for testing
type KvStore map[string]storage.Value

// Empty checks if there are any keys in the store
func (k *KvStore) Empty() bool {
	return len(*k) == 0
}

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
		return nil, nil
	}
	v, ok := m.Data[key.String()]
	if !ok {
		return storage.Value{}, nil
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
func (m *MockKeyValueStore) List(startingKey storage.Key, limit storage.Limit) (storage.Keys, error) {
	m.ListCalled++
	keys := storage.Keys{}
	keySlice := mapIntoSlice(m.Data)
	started := false

	if startingKey == nil {
		started = true
	}
	for _, key := range keySlice {
		if !started && key == string(startingKey) {
			keys = append(keys, storage.Key(key))
			started = true
			continue
		}
		if started {
			if len(keys) == int(limit) {
				break
			}
			keys = append(keys, storage.Key(key))
		}
	}
	return keys, nil
}

func mapIntoSlice(data KvStore) []string {
	keySlice := make([]string, len(data))
	i := 0
	for k := range data {
		keySlice[i] = k
		i++
	}
	sort.Strings(keySlice)
	return keySlice
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

// RedisAddress is the address used by redis for tests
const RedisAddress = "127.0.0.1:6379"

// EnsureRedis attempts to start the `redis-server` binary
// Tests that want to use Redis should configure the application to use
// the RedisAddress variable
func EnsureRedis(t *testing.T) (_ RedisDone) {
	index, _ := randomHex(5)
	redisRefs[index] = true

	if testRedis.started != true {
		conn, err := net.Dial("tcp", "127.0.0.1:6379")
		if err != nil {
			testRedis.start(t)
		} else {
			testRedis.started = true
			n, err := conn.Write([]byte("*1\r\n$8\r\nflushall\r\n"))
			if err != nil {
				log.Fatalf("Failed to request flush of existing redis keys: error %s\n", err)
			}
			b := make([]byte, 5)
			n, err = conn.Read(b)
			if err != nil {
				log.Fatalf("Failed to flush existing redis keys: error %s\n", err)
			}
			if n != len(b) || !bytes.Equal(b, []byte("+OK\r\n")) {
				log.Fatalf("Failed to flush existing redis keys: Unexpected response %s\n", b)
			}
			conn.Close()
		}
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
	if r.cmd == nil {
		return
	}
	if err := r.cmd.Process.Kill(); err != nil {
		log.Printf("Failed to kill process: %s\n", err)
	}
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
