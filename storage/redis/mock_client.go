package redis

import (
	"errors"
	"time"
)

type mockRedisClient struct {
	data       map[string][]byte
	getCalled  int
	setCalled  int
	pingCalled int
}

var ErrMissingKey = errors.New("missing")
var ErrForced = errors.New("error forced by using 'error' key in mock")

func (m *mockRedisClient) Get(key string) ([]byte, error) {
	m.getCalled++
	if key == "error" {
		return []byte{}, ErrForced
	}
	v, ok := m.data[key]
	if !ok {
		return []byte{}, ErrMissingKey
	}

	return v, nil
}

func (m *mockRedisClient) Set(key string, value []byte, ttl time.Duration) error {
	m.setCalled++
	m.data[key] = value
	return nil
}

func (m *mockRedisClient) Ping() error {
	m.pingCalled++
	return nil
}

func newMockRedisClient(d map[string][]byte) *mockRedisClient {
	return &mockRedisClient{
		data:       d,
		getCalled:  0,
		setCalled:  0,
		pingCalled: 0,
	}
}
