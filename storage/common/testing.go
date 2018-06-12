// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package storage

import (
	"flag"
	"path/filepath"
	"os/exec"
	"os"
	"fmt"
	"time"
	"encoding/hex"
	"crypto/rand"

	"github.com/zeebo/errs"
)

type MockStorageClient struct {
	Data         map[string][]byte
	GetCalled    int
	PutCalled    int
	ListCalled   int
	DeleteCalled int
	CloseCalled  int
	PingCalled   int
}

type TestRedisDone func()
type TestRedisServer struct {
	cmd     *exec.Cmd
	started bool
}

var (
	// ErrMissingKey is the error returned if a key is not in the mock store
	ErrMissingKey = errs.New("missing")

	// ErrForced is the error returned when the forced error flag is passed to mock an error
	ErrForced = errs.New("error forced by using 'error' key in mock")

	redisRefs = map[string]bool{}
	testRedis = &TestRedisServer{
		started: false,
	}
)

func (m *MockStorageClient) Get(key []byte) ([]byte, error) {
	m.GetCalled++
	if string(key) == "error" {
		return []byte{}, ErrForced
	}
	v, ok := m.Data[string(key)]
	if !ok {
		return []byte{}, ErrMissingKey
	}

	return v, nil
}

func (m *MockStorageClient) Put(key, value []byte) error {
	m.PutCalled++
	m.Data[string(key)] = value
	return nil
}

func (m *MockStorageClient) Delete(key []byte) error {
	m.DeleteCalled++
	delete(m.Data, string(key))
	return nil
}

func (m *MockStorageClient) List() (_ [][]byte, _ error) {
	m.ListCalled++
	keys := [][]byte{}
	for k := range m.Data {
		keys = append(keys, []byte(k))
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

func NewMockStorageClient(d map[string][]byte) *MockStorageClient {
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

func EnsureRedis() (_ TestRedisDone) {
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
			count += 1
		}
	}

	return count
}

func (r *TestRedisServer) start() {
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

func (r *TestRedisServer) stop() {
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
