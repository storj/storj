// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"testing"

	"storj.io/storj/internal/test"
	"storj.io/storj/storage"
)

type RedisClientTest struct {
	*testing.T
	c storage.KeyValueStore
}

func NewRedisClientTest(t *testing.T) *RedisClientTest {
	kv := make(test.KvStore)
	c := test.NewMockKeyValueStore(kv)
	return &RedisClientTest{
		T: t,
		c: c,
	}
}

func (rt *RedisClientTest) Close() {
	rt.c.Close()
}

func (rt *RedisClientTest) HandleErr(err error, msg string) {
	rt.Error(msg)
	if err != nil {
		panic(err)
	}
	panic(msg)
}

func TestListWithoutStartKey(t *testing.T) {
	rt := NewRedisClientTest(t)
	defer rt.Close()

	if err := rt.c.Put(storage.Key([]byte("path/1")), []byte("pointer1")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/2")), []byte("pointer2")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/3")), []byte("pointer3")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}

	_, err := rt.c.List(nil, storage.Limit(3))
	if err != nil {
		rt.HandleErr(err, "Failed to list")
	}
}

func TestListWithStartKey(t *testing.T) {
	rt := NewRedisClientTest(t)
	defer rt.Close()

	if err := rt.c.Put(storage.Key([]byte("path/1")), []byte("pointer1")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/2")), []byte("pointer2")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/3")), []byte("pointer3")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/4")), []byte("pointer4")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/5")), []byte("pointer5")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}

	_, err := rt.c.List([]byte("path/2"), storage.Limit(2))
	if err != nil {
		rt.HandleErr(err, "Failed to list")
	}
}
