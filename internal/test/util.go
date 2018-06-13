// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package test

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/zeebo/errs"
	"storj.io/storj/storage"
)

// KvStore is an in-memory, crappy key/value store type for testing
type KvStore map[string]storage.Value

// MockStorageClient is a `KeyValueStore` type used for testing (see storj.io/storj/storage/common.go)
type MockStorageClient struct {
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
	// ErrMissingKey is the error returned if a key is not in the mock store
	ErrMissingKey = errs.New("missing")

	// ErrForced is the error returned when the forced error flag is passed to mock an error
	ErrForced = errs.New("error forced by using 'error' key in mock")

	redisRefs = map[string]bool{}
	testRedis = &RedisServer{
		started: false,
	}
)

// Get looks up the provided key from the MockStorageClient returning either an error or the result.
func (m *MockStorageClient) Get(key storage.Key) (storage.Value, error) {
	m.GetCalled++
	if key.String() == "error" {
		return storage.Value{}, ErrForced
	}
	v, ok := m.Data[key.String()]
	if !ok {
		return storage.Value{}, ErrMissingKey
	}

	return v, nil
}

// Put adds a value to the provided key in the MockStorageClient, returning an error on failure.
func (m *MockStorageClient) Put(key storage.Key, value storage.Value) error {
	m.PutCalled++
	m.Data[key.String()] = value
	return nil
}

func (m *MockStorageClient) Delete(key storage.Key) error {
	m.DeleteCalled++
	delete(m.Data, key.String())
	return nil
}

func (m *MockStorageClient) List() (_ storage.Keys, _ error) {
	m.ListCalled++
	keys := storage.Keys{}
	for k := range m.Data {
		keys = append(keys, storage.Key(k))
	}

	return keys, nil
}

func (m *MockStorageClient) Close() error {
	m.CloseCalled++
	return nil
}

func (m *MockStorageClient) Ping() error {
	m.PingCalled++
	return nil
}

func NewMockStorageClient(d KvStore) *MockStorageClient {
	return &MockStorageClient{
		Data:         d,
		GetCalled:    0,
		PutCalled:    0,
		ListCalled:   0,
		DeleteCalled: 0,
		CloseCalled:  0,
		PingCalled:   0,
	}
}

func EnsureRedis() (_ RedisDone) {
	flag.Set("redisAddress", "127.0.0.1:6379")

	index, _ := randomHex(5)
	redisRefs[index] = true

	if testRedis.started != true {
		testRedis.start()
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

func (r *RedisServer) start() {
	r.cmd = &exec.Cmd{}
	cmd := r.cmd

	logPath, err := filepath.Abs("test_redis-server.log")
	if err != nil {
		panic(err)
	}

	binPath, err := exec.LookPath("redis-server")
	if err != nil {
		panic(err)
	}

	log, err := os.Create(logPath)
	if err != nil {
		panic(err)
	}

	cmd.Path = binPath
	cmd.Stdout = log

	go func() {
		r.started = true

		if err := cmd.Run(); err != nil {
			// TODO(bryanchriswhite) error checking
		}
	}()

	fmt.Printf("starting redis; sleeping")
	for range []int{0, 1} {
		fmt.Printf(".")
		time.Sleep(1 * time.Second)
	}
	fmt.Printf("done\n")
}

func (r *RedisServer) stop() {
	r.started = false
	if err := r.cmd.Process.Kill(); err != nil {
		// panic(err)
	}
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
